/*
Copyright 2022 Huawei Technologies Co., Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	mindsporev1 "ms-operator/pkg/apis/v1"

	"github.com/go-logr/logr"
	commonv1 "github.com/kubeflow/common/pkg/apis/common/v1"
	"github.com/kubeflow/common/pkg/controller.v1/common"
	"github.com/kubeflow/common/pkg/controller.v1/control"
	"github.com/kubeflow/common/pkg/controller.v1/expectation"
	"github.com/kubeflow/common/pkg/core"
	commonutil "github.com/kubeflow/common/pkg/util"
	"github.com/kubeflow/common/pkg/util/k8sutil"
	utillabels "github.com/kubeflow/common/pkg/util/labels"
	train_util "github.com/kubeflow/common/pkg/util/train"
	trainingoperatorcommon "github.com/kubeflow/training-operator/pkg/common"
	"github.com/kubeflow/training-operator/pkg/common/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	kubeclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"volcano.sh/apis/pkg/apis/scheduling/v1beta1"
	volcanoclient "volcano.sh/apis/pkg/client/clientset/versioned"
)

const (
	// msJobSucceededReason is added in a msjob when it is succeeded.
	msJobSucceededReason = "MSJobSucceeded"
	// msJobRunningReason is added in a msjob when it is running.
	msJobRunningReason = "MSJobRunning"
	// msJobFailedReason is added in a msjob when it is failed.
	msJobFailedReason = "MSJobFailed"
	// msJobRestarting is added in a msjob when it is restarting.
	msJobRestartingReason = "MSJobRestarting"

	FailedDeleteJobReason     = "FailedDeleteJob"
	SuccessfulDeleteJobReason = "SuccessfulDeleteJob"

	controllerName = "msjob-controller"

	// labels for pods and servers.
	msReplicaTypeLabel  = "replica-type"
	msReplicaIndexLabel = "replica-index"
	// volcanoTaskSpecKey task spec key used in pod annotation when EnableGangScheduling is true
	volcanoTaskSpecKey = "volcano.sh/task-spec"

	// gang scheduler name.
	gangSchedulerName = "volcano"
	//the environment variable name of Mindspore cluster spec.
	msServerNum = "MS_SERVER_NUM"
	msWorkerNum = "MS_WORKER_NUM"
	msSchedHost = "MS_SCHED_HOST"
	msSchedPort = "MS_SCHED_PORT"
	msRole      = "MS_ROLE"
	msNodeId    = "MS_NODE_ID"

	// exitedWithCodeReason is the normal reason when the pod is exited because of the exit code.
	exitedWithCodeReason = "ExitedWithCode"
	// podTemplateRestartPolicyReason is the warning reason when the restart
	// policy is set in pod template.
	podTemplateRestartPolicyReason = "SettedPodTemplateRestartPolicy"
	// podTemplateSchedulerNameReason is the warning reason when other scheduler name is set
	// in pod templates with gang-scheduling enabled
	podTemplateSchedulerNameReason = "SettedPodTemplateSchedulerName"
	// gangSchedulingPodGroupAnnotation is the annotation key used by batch schedulers
	gangSchedulingPodGroupAnnotation = "scheduling.k8s.io/group-name"
)

var (
	msRoleMap = map[string]string{
		"scheduler": "MS_SCHED",
		"ps":        "MS_PSERVER",
		"worker":    "MS_WORKER",
	}
	succeededServiceCreationCount = promauto.With(prometheus.NewRegistry()).NewCounter(prometheus.CounterOpts{
		Name: "succeeded_service_creation_total",
		Help: "The total number of succeeded service creation",
	})
	failedServiceCreationCount = promauto.With(prometheus.NewRegistry()).NewCounter(prometheus.CounterOpts{
		Name: "failed_service_creation_total",
		Help: "The total number of failed service creation",
	})
)

func NewReconciler(mgr manager.Manager, enableGangScheduling bool) *MSJobReconciler {
	r := &MSJobReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		recorder: mgr.GetEventRecorderFor(controllerName),
		Log:      log.Log,
	}

	cfg := mgr.GetConfig()
	kubeClientSet := kubeclientset.NewForConfigOrDie(cfg)
	volcanoClientSet := volcanoclient.NewForConfigOrDie(cfg)
	sharedInformers := informers.NewSharedInformerFactory(kubeClientSet, 0)
	priorityClassInformer := sharedInformers.Scheduling().V1beta1().PriorityClasses()

	r.JobController = common.JobController{
		Controller:                  r,
		Expectations:                expectation.NewControllerExpectations(),
		Config:                      common.JobControllerConfiguration{EnableGangScheduling: enableGangScheduling},
		WorkQueue:                   &util.FakeWorkQueue{},
		Recorder:                    r.recorder,
		KubeClientSet:               kubeClientSet,
		VolcanoClientSet:            volcanoClientSet,
		PriorityClassLister:         priorityClassInformer.Lister(),
		PriorityClassInformerSynced: priorityClassInformer.Informer().HasSynced,
		PodControl:                  control.RealPodControl{KubeClient: kubeClientSet, Recorder: r.recorder},
		ServiceControl:              control.RealServiceControl{KubeClient: kubeClientSet, Recorder: r.recorder},
	}

	return r
}

// MSJobReconciler reconciles a MSJob object
type MSJobReconciler struct {
	common.JobController
	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
	Log      logr.Logger
}

//+kubebuilder:rbac:groups=mindspore.gitee.com,resources=msjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mindspore.gitee.com,resources=msjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mindspore.gitee.com,resources=msjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the MSJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *MSJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	logger := r.Log.WithValues(mindsporev1.Singular, req.NamespacedName)

	msjob := &mindsporev1.MSJob{}
	err := r.Get(ctx, req.NamespacedName, msjob)
	if err != nil {
		logger.Info(err.Error(), "unable to fetch MSJob", req.NamespacedName.String())
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err = validateV1ReplicaSpecs(msjob.Spec.MSReplicaSpecs); err != nil {
		logger.Info(err.Error(), "MSJob failed validation", req.NamespacedName.String())
	}

	// Check if reconciliation is needed
	jobKey, err := common.KeyFunc(msjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get jobKey for job object %#v: %v", msjob, err))
	}

	replicaTypes := util.GetReplicaTypes(msjob.Spec.MSReplicaSpecs)
	needReconcile := util.SatisfiedExpectations(r.Expectations, jobKey, replicaTypes)

	if !needReconcile || msjob.GetDeletionTimestamp() != nil {
		logger.Info("reconcile cancelled, job does not need to do reconcile or has been deleted",
			"sync", needReconcile, "deleted", msjob.GetDeletionTimestamp() != nil)
		return ctrl.Result{}, nil
	}

	// Set default priorities to msjob
	r.Scheme.Default(msjob)

	// Use common to reconcile the job related pod and service
	err = r.ReconcileJobs(msjob, msjob.Spec.MSReplicaSpecs, msjob.Status, &msjob.Spec.RunPolicy)
	if err != nil {
		logrus.Warnf("Reconcile MindSpore Job error %v", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MSJobReconciler) SetupWithManager(mgr ctrl.Manager) error {

	c, err := controller.New(r.ControllerName(), mgr, controller.Options{
		Reconciler: r,
	})

	if err != nil {
		return err
	}

	// using onOwnerCreateFunc is easier to set defaults
	if err = c.Watch(&source.Kind{Type: &mindsporev1.MSJob{}}, &handler.EnqueueRequestForObject{},
		predicate.Funcs{CreateFunc: r.onOwnerCreateFunc()},
	); err != nil {
		return err
	}

	// inject watching for job related pod
	if err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mindsporev1.MSJob{},
	}, predicate.Funcs{
		CreateFunc: util.OnDependentCreateFunc(r.Expectations),
		UpdateFunc: util.OnDependentUpdateFunc(&r.JobController),
		DeleteFunc: util.OnDependentDeleteFunc(r.Expectations),
	}); err != nil {
		return err
	}

	// inject watching for job related service
	if err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &mindsporev1.MSJob{},
	}, predicate.Funcs{
		CreateFunc: util.OnDependentCreateFunc(r.Expectations),
		UpdateFunc: util.OnDependentUpdateFunc(&r.JobController),
		DeleteFunc: util.OnDependentDeleteFunc(r.Expectations),
	}); err != nil {
		return err
	}

	return nil
}

func (jc *MSJobReconciler) ReconcileJobs(
	job interface{},
	replicas map[commonv1.ReplicaType]*commonv1.ReplicaSpec,
	jobStatus commonv1.JobStatus,
	runPolicy *commonv1.RunPolicy) error {

	metaObject, ok := job.(metav1.Object)
	jobName := metaObject.GetName()
	if !ok {
		return fmt.Errorf("job is not of type metav1.Object")
	}
	runtimeObject, ok := job.(runtime.Object)
	if !ok {
		return fmt.Errorf("job is not of type runtime.Object")
	}
	jobKey, err := common.KeyFunc(job)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for job object %#v: %v", job, err))
		return err
	}
	// Reset expectations
	// 1. Since `ReconcileJobs` is called, we expect that previous expectations are all satisfied,
	//    and it's safe to reset the expectations
	// 2. Reset expectations can avoid dirty data such as `expectedDeletion = -1`
	//    (pod or service was deleted unexpectedly)
	jc.ResetExpectations(jobKey, replicas)

	logrus.Infof("Reconciling for job %s", metaObject.GetName())
	pods, err := jc.Controller.GetPodsForJob(job)
	if err != nil {
		logrus.Warnf("GetPodsForJob error %v", err)
		return err
	}

	services, err := jc.Controller.GetServicesForJob(job)
	if err != nil {
		logrus.Warnf("GetServicesForJob error %v", err)
		return err
	}

	oldStatus := jobStatus.DeepCopy()
	if commonutil.IsSucceeded(jobStatus) || commonutil.IsFailed(jobStatus) {
		// If the Job is succeed or failed, delete all pods and services.
		if err := jc.DeletePodsAndServices(runPolicy, job, pods); err != nil {
			return err
		}

		if jc.Config.EnableGangScheduling {
			jc.Recorder.Event(runtimeObject, v1.EventTypeNormal, "JobTerminated", "Job has been terminated. Deleting PodGroup")
			if err := jc.DeletePodGroup(metaObject); err != nil {
				jc.Recorder.Eventf(runtimeObject, v1.EventTypeWarning, "FailedDeletePodGroup", "Error deleting: %v", err)
				return err
			} else {
				jc.Recorder.Eventf(runtimeObject, v1.EventTypeNormal, "SuccessfulDeletePodGroup", "Deleted PodGroup: %v", jobName)
			}
		}

		if err := jc.CleanupJob(runPolicy, jobStatus, job); err != nil {
			return err
		}

		// At this point the pods may have been deleted.
		// 1) If the job succeeded, we manually set the replica status.
		// 2) If any replicas are still active, set their status to succeeded.
		if commonutil.IsSucceeded(jobStatus) {
			for rtype := range jobStatus.ReplicaStatuses {
				jobStatus.ReplicaStatuses[rtype].Succeeded += jobStatus.ReplicaStatuses[rtype].Active
				jobStatus.ReplicaStatuses[rtype].Active = 0
			}
		}

		// No need to update the job status if the status hasn't changed since last time.
		if !reflect.DeepEqual(*oldStatus, jobStatus) {
			return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
		}

		return nil
	}

	// retrieve the previous number of retry
	previousRetry := jc.WorkQueue.NumRequeues(jobKey)

	activePods := k8sutil.FilterActivePods(pods)

	core.RecordAbnormalPods(activePods, runtimeObject, jc.Recorder)

	active := int32(len(activePods))
	failed := k8sutil.FilterPodCount(pods, v1.PodFailed)
	totalReplicas := k8sutil.GetTotalReplicas(replicas)
	prevReplicasFailedNum := k8sutil.GetTotalFailedReplicas(jobStatus.ReplicaStatuses)

	var failureMessage string
	jobExceedsLimit := false
	exceedsBackoffLimit := false
	pastBackoffLimit := false

	if runPolicy.BackoffLimit != nil {
		jobHasNewFailure := failed > prevReplicasFailedNum
		// new failures happen when status does not reflect the failures and active
		// is different than parallelism, otherwise the previous controller loop
		// failed updating status so even if we pick up failure it is not a new one
		exceedsBackoffLimit = jobHasNewFailure && (active != totalReplicas) &&
			(int32(previousRetry)+1 > *runPolicy.BackoffLimit)

		pastBackoffLimit, err = jc.PastBackoffLimit(jobName, runPolicy, replicas, pods)
		if err != nil {
			return err
		}
	}

	if exceedsBackoffLimit || pastBackoffLimit {
		// check if the number of pod restart exceeds backoff (for restart OnFailure only)
		// OR if the number of failed jobs increased since the last syncJob
		jobExceedsLimit = true
		failureMessage = fmt.Sprintf("Job %s has failed because it has reached the specified backoff limit", jobName)
	} else if jc.PastActiveDeadline(runPolicy, jobStatus) {
		failureMessage = fmt.Sprintf("Job %s has failed because it was active longer than specified deadline", jobName)
		jobExceedsLimit = true
	}

	if jobExceedsLimit {
		// Set job completion time before resource cleanup
		if jobStatus.CompletionTime == nil {
			now := metav1.Now()
			jobStatus.CompletionTime = &now
		}

		// If the Job exceeds backoff limit or is past active deadline
		// delete all pods and services, then set the status to failed
		if err := jc.DeletePodsAndServices(runPolicy, job, pods); err != nil {
			return err
		}

		if err := jc.CleanupJob(runPolicy, jobStatus, job); err != nil {
			return err
		}

		if jc.Config.EnableGangScheduling {
			jc.Recorder.Event(runtimeObject, v1.EventTypeNormal, "JobTerminated", "Job has been terminated. Deleting PodGroup")
			if err := jc.DeletePodGroup(metaObject); err != nil {
				jc.Recorder.Eventf(runtimeObject, v1.EventTypeWarning, "FailedDeletePodGroup", "Error deleting: %v", err)
				return err
			} else {
				jc.Recorder.Eventf(runtimeObject, v1.EventTypeNormal, "SuccessfulDeletePodGroup", "Deleted PodGroup: %v", jobName)
			}
		}

		jc.Recorder.Event(runtimeObject, v1.EventTypeNormal, commonutil.JobFailedReason, failureMessage)

		if err := commonutil.UpdateJobConditions(&jobStatus, commonv1.JobFailed, commonutil.JobFailedReason, failureMessage); err != nil {
			logrus.Infof("Append job condition error: %v", err)
			return err
		}

		return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
	} else {
		// General cases which need to reconcile
		if jc.Config.EnableGangScheduling {
			minMember := totalReplicas
			queue := ""
			priorityClass := ""
			var minResources *v1.ResourceList

			if runPolicy.SchedulingPolicy != nil {
				if runPolicy.SchedulingPolicy.MinAvailable != nil {
					minMember = *runPolicy.SchedulingPolicy.MinAvailable
				}

				if runPolicy.SchedulingPolicy.Queue != "" {
					queue = runPolicy.SchedulingPolicy.Queue
				}

				if runPolicy.SchedulingPolicy.PriorityClass != "" {
					priorityClass = runPolicy.SchedulingPolicy.PriorityClass
				}

				if runPolicy.SchedulingPolicy.MinResources != nil {
					minResources = runPolicy.SchedulingPolicy.MinResources
				}
			}

			if minResources == nil {
				minResources = common.CalcPGMinResources(minMember, replicas, jc.PriorityClassLister.Get)
			}

			pgSpec := v1beta1.PodGroupSpec{
				MinMember:         minMember,
				Queue:             queue,
				PriorityClassName: priorityClass,
				MinResources:      minResources,
			}

			syncReplicas := true
			pg, err := jc.SyncPodGroup(metaObject, pgSpec)
			if err != nil {
				logrus.Warnf("Sync PodGroup %v: %v", jobKey, err)
				syncReplicas = false
			}

			// Delay pods creation until podgroup status is inqueue
			if pg == nil || pg.Status.Phase == "" || pg.Status.Phase == v1beta1.PodGroupPending {
				logrus.Warnf("PodGroup %v unschedulable", jobKey)
				syncReplicas = false
			}

			if !syncReplicas {
				now := metav1.Now()
				jobStatus.LastReconcileTime = &now

				// Update job status here to trigger a new reconciliation
				return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
			}
		}

		// Diff current active pods/services with replicas.
		// Create Scheduler pod and service first, and then Create the others pods.
		schedulerRoleSpec, hasSchedulerRole := replicas[mindsporev1.MSReplicaTypeScheduler]
		if hasSchedulerRole {
			err := jc.Controller.ReconcileServices(metaObject, services, mindsporev1.MSReplicaTypeScheduler, schedulerRoleSpec)
			if err != nil {
				logrus.Warnf("ReconcileServices error %v", err)
				return err
			}

			err = jc.Controller.ReconcilePods(metaObject, &jobStatus, pods, mindsporev1.MSReplicaTypeScheduler, schedulerRoleSpec, replicas)
			if err != nil {
				logrus.Warnf("ReconcilePods error %v", err)
				return err
			}
		}

		for rtype, spec := range replicas {
			if rtype != mindsporev1.MSReplicaTypeScheduler {
				err := jc.Controller.ReconcilePods(metaObject, &jobStatus, pods, rtype, spec, replicas)
				if err != nil {
					logrus.Warnf("ReconcilePods error %v", err)
					return err
				}
			}
		}
	}

	err = jc.Controller.UpdateJobStatus(job, replicas, &jobStatus)
	if err != nil {
		logrus.Warnf("UpdateJobStatus error %v", err)
		return err
	}
	// No need to update the job status if the status hasn't changed since last time.
	if !reflect.DeepEqual(*oldStatus, jobStatus) {
		return jc.Controller.UpdateJobStatusInApiServer(job, &jobStatus)
	}
	return nil
}

func (r *MSJobReconciler) ControllerName() string {
	return controllerName
}

func (r *MSJobReconciler) GetAPIGroupVersionKind() schema.GroupVersionKind {
	return mindsporev1.GroupVersion.WithKind(mindsporev1.Kind)
}

func (r *MSJobReconciler) GetAPIGroupVersion() schema.GroupVersion {
	return mindsporev1.GroupVersion
}

func (r *MSJobReconciler) GetGroupNameLabelValue() string {
	return mindsporev1.GroupVersion.Group
}

func (r *MSJobReconciler) GetJobFromInformerCache(namespace, name string) (metav1.Object, error) {
	msjob := &mindsporev1.MSJob{}
	err := r.Get(context.Background(), types.NamespacedName{
		Namespace: namespace, Name: name,
	}, msjob)
	return msjob, err
}

func (r *MSJobReconciler) GetJobFromAPIClient(namespace, name string) (metav1.Object, error) {
	job := &mindsporev1.MSJob{}

	clientReader, err := util.GetDelegatingClientFromClient(r.Client)
	if err != nil {
		return nil, err
	}
	err = clientReader.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, job)
	if err != nil {
		if errors.IsNotFound(err) {
			logrus.Error(err, "mindspore job not found", "namespace", namespace, "name", name)
		} else {
			logrus.Error(err, "failed to get job from api-server", "namespace", namespace, "name", name)
		}
		return nil, err
	}
	return job, nil
}

// GetPodsForJob returns the set of pods that this job should manage.
// It also reconciles ControllerRef by adopting/orphaning.
// Note that the returned Pods are pointers into the cache.
func (r *MSJobReconciler) GetPodsForJob(jobObject interface{}) ([]*corev1.Pod, error) {
	job, ok := jobObject.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("job is not of type metav1.Object")
	}

	// Create selector.
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: r.GenLabels(job.GetName()),
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't convert Job selector: %v", err)
	}
	// List all pods to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	podlist := &corev1.PodList{}
	err = r.List(context.Background(), podlist,
		client.MatchingLabelsSelector{Selector: selector}, client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, err
	}

	pods := util.ConvertPodList(podlist.Items)

	// If any adoptions are attempted, we should first recheck for deletion
	// with an uncached quorum read sometime after listing Pods (see #42639).
	canAdoptFunc := common.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := r.Controller.GetJobFromAPIClient(job.GetNamespace(), job.GetName())
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != job.GetUID() {
			return nil, fmt.Errorf("original Job %v/%v is gone: got uid %v, wanted %v", job.GetNamespace(), job.GetName(), fresh.GetUID(), job.GetUID())
		}
		return fresh, nil
	})
	cm := control.NewPodControllerRefManager(r.PodControl, job, selector, r.Controller.GetAPIGroupVersionKind(), canAdoptFunc)
	return cm.ClaimPods(pods)
}

// GetServicesForJob returns the set of services that this job should manage.
// It also reconciles ControllerRef by adopting/orphaning.
// Note that the returned services are pointers into the cache.
func (r *MSJobReconciler) GetServicesForJob(jobObject interface{}) ([]*corev1.Service, error) {
	job, ok := jobObject.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("job is not of type metav1.Object")
	}

	// Create selector
	selector, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
		MatchLabels: r.GenLabels(job.GetName()),
	})

	if err != nil {
		return nil, fmt.Errorf("couldn't convert Job selector: %v", err)
	}
	// List all services to include those that don't match the selector anymore
	// but have a ControllerRef pointing to this controller.
	svclist := &corev1.ServiceList{}
	err = r.List(context.Background(), svclist,
		client.MatchingLabelsSelector{Selector: selector}, client.InNamespace(job.GetNamespace()))
	if err != nil {
		return nil, fmt.Errorf("couldn't get Service: %v", err)
	}

	// If any adoptions are attempted, we should first recheck for deletion
	// with an uncached quorum read sometime after listing services (see #42639).
	canAdoptFunc := common.RecheckDeletionTimestamp(func() (metav1.Object, error) {
		fresh, err := r.GetJobFromInformerCache(job.GetNamespace(), job.GetName())
		if err != nil {
			return nil, err
		}
		if fresh.GetUID() != job.GetUID() {
			return nil, fmt.Errorf("original Job %v/%v is gone: got uid %v, wanted %v", job.GetNamespace(), job.GetName(), fresh.GetUID(), job.GetUID())
		}
		return fresh, nil
	})
	cm := control.NewServiceControllerRefManager(r.ServiceControl, job, selector, r.Controller.GetAPIGroupVersionKind(), canAdoptFunc)

	services := util.ConvertServiceList(svclist.Items)

	return cm.ClaimServices(services)
}

func (r *MSJobReconciler) DeleteJob(job interface{}) error {
	msJob, ok := job.(*mindsporev1.MSJob)
	if !ok {
		return fmt.Errorf("%v is not a type of MSJob", msJob)
	}

	log := commonutil.LoggerForJob(msJob)
	if err := r.Delete(context.Background(), msJob); err != nil {
		r.recorder.Eventf(msJob, v1.EventTypeWarning, FailedDeleteJobReason, "Error deleting: %v", err)
		log.Errorf("failed to delete job %s/%s, %v", msJob.Namespace, msJob.Name, err)
		return err
	}

	r.recorder.Eventf(msJob, v1.EventTypeNormal, SuccessfulDeleteJobReason, "Deleted job: %v", msJob.Name)
	log.Infof("job %s/%s has been deleted", msJob.Namespace, msJob.Name)
	trainingoperatorcommon.DeletedJobsCounterInc(msJob.Namespace, mindsporev1.FrameworkName)
	return nil
}

func (r *MSJobReconciler) UpdateJobStatus(job interface{}, replicas map[commonv1.ReplicaType]*commonv1.ReplicaSpec, jobStatus *commonv1.JobStatus) error {
	msJob, ok := job.(*mindsporev1.MSJob)
	if !ok {
		return fmt.Errorf("%v is not a type of MSJob", msJob)
	}

	msJobKey, err := common.KeyFunc(msJob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for msjob object %#v: %v", msJob, err))
		return err
	}

	logger := commonutil.LoggerForJob(msJob)

	worker0Completed, err := r.IsWorker0Completed(msJob, replicas)
	if err != nil {
		logger.Warnf("check if worker 0 completed error %v", err)
		return err
	}

	// Set StartTime.
	if jobStatus.StartTime == nil {
		now := metav1.Now()
		jobStatus.StartTime = &now
		// enqueue a sync to check if job past ActiveDeadlineSeconds
		if msJob.Spec.RunPolicy.ActiveDeadlineSeconds != nil {
			logger.Infof("Job with ActiveDeadlineSeconds will sync after %d seconds", *msJob.Spec.RunPolicy.ActiveDeadlineSeconds)
			r.WorkQueue.AddAfter(msJobKey, time.Duration(*msJob.Spec.RunPolicy.ActiveDeadlineSeconds)*time.Second)
		}
	}
	// iterate the replica spec based on this order
	allTypes := []commonv1.ReplicaType{
		mindsporev1.MSReplicaTypeScheduler,
		mindsporev1.MSReplicaTypePS,
		mindsporev1.MSReplicaTypeWorker,
	}
	for _, rtype := range allTypes {
		if replicas[rtype] == nil {
			continue
		}
		spec := replicas[rtype]
		status := jobStatus.ReplicaStatuses[rtype]

		// Expect to have `replicas - succeeded` pods alive.
		succeeded := status.Succeeded
		expected := *(spec.Replicas) - succeeded
		running := status.Active
		failed := status.Failed

		logger.Infof("MSJob=%s/%s, ReplicaType=%s expected=%d, running=%d, failed=%d",
			msJob.Namespace, msJob.Name, rtype, expected, running, failed)

		if isWorker(rtype) {
			// Leave a succeeded condition for the following two cases:
			// 1. If default success policy is used and worker 0 has completed.
			// 2. If `SuccessPolicyAllWorkers` success policy is used and all workers are succeeded.
			if expected == 0 || (worker0Completed && *msJob.Spec.SuccessPolicy != mindsporev1.SuccessPolicyAllWorkers) {
				msg := fmt.Sprintf("MSJob %s/%s successfully completed.",
					msJob.Namespace, msJob.Name)
				r.recorder.Event(msJob, corev1.EventTypeNormal, msJobSucceededReason, msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				err := commonutil.UpdateJobConditions(jobStatus,
					commonv1.JobSucceeded, msJobSucceededReason, msg)
				if err != nil {
					commonutil.LoggerForJob(msJob).Infof("Append msjob condition error: %v", err)
					return err
				}
				trainingoperatorcommon.SuccessfulJobsCounterInc(msJob.Namespace, mindsporev1.FrameworkName)
			} else if running > 0 {
				// Some workers are still running, leave a running condition.
				msg := fmt.Sprintf("MSJob %s/%s is running.",
					msJob.Namespace, msJob.Name)
				err := commonutil.UpdateJobConditions(jobStatus, commonv1.JobRunning, msJobRunningReason, msg)
				if err != nil {
					commonutil.LoggerForJob(msJob).Infof("Append msjob condition error: %v", err)
					return err
				}
			}
		}

		if failed > 0 {
			restart := false
			for _, condition := range jobStatus.Conditions {
				if condition.Type == commonv1.JobRestarting {
					restart = true
				}
			}

			if restart {
				// job is restarting, no need to set it failed
				// we know it because we update the status condition when reconciling the replicas
				trainingoperatorcommon.RestartedJobsCounterInc(msJob.Namespace, mindsporev1.FrameworkName)
			} else {
				if msJob.Spec.EnableDynamicWorker && isWorker(rtype) {
					commonutil.LoggerForJob(msJob).Infof("MSJob %s/%s continues regardless %d Worker replica(s) failed as enableDynamicWorker is set true.",
						msJob.Namespace, msJob.Name, failed)
					continue
				}
				msg := fmt.Sprintf("MSJob %s/%s has failed because %d %s replica(s) failed.",
					msJob.Namespace, msJob.Name, failed, rtype)
				r.recorder.Event(msJob, corev1.EventTypeNormal, msJobFailedReason, msg)
				if jobStatus.CompletionTime == nil {
					now := metav1.Now()
					jobStatus.CompletionTime = &now
				}
				err := commonutil.UpdateJobConditions(jobStatus,
					commonv1.JobFailed, msJobFailedReason, msg)
				if err != nil {
					commonutil.LoggerForJob(msJob).Infof("Append msjob condition error: %v", err)
					return err
				}
				trainingoperatorcommon.FailedJobsCounterInc(msJob.Namespace, mindsporev1.FrameworkName)
			}
		}
	}
	// we assign the jobStatus to the msJob.Status for testing purpose
	// it won't effect the main reconcile logic
	// because we already use oldStatus := jobStatus.DeepCopy() to record the oldStatus
	// and use !reflect.DeepEqual(*oldStatus, jobStatus) to decide whether to update the msJob or not
	msJob.Status = *jobStatus.DeepCopy()

	return nil
}

func (r *MSJobReconciler) UpdateJobStatusInApiServer(job interface{}, jobStatus *commonv1.JobStatus) error {
	msJob, ok := job.(*mindsporev1.MSJob)
	if !ok {
		return fmt.Errorf("%v is not a type of MSJob", msJob)
	}

	startTime := time.Now()
	logger := commonutil.LoggerForJob(msJob)
	defer func() {
		logger.Infof("Finished updating MSJobs Status %q (%v)",
			msJob.Name, time.Since(startTime))
	}()

	msJob = msJob.DeepCopy()
	msJob.Status = *jobStatus.DeepCopy()

	result := r.Status().Update(context.Background(), msJob)

	if result != nil {
		r.Log.WithValues("msjob", types.NamespacedName{
			Namespace: msJob.GetNamespace(),
			Name:      msJob.GetName(),
		})
		return result
	}

	return nil
}

// Set Envs for MSJob
func (r *MSJobReconciler) SetClusterSpec(job interface{}, podTemplate *corev1.PodTemplateSpec, rtype, index string) error {
	msjob, ok := job.(*mindsporev1.MSJob)
	if !ok {
		return fmt.Errorf("%v is not a type of MSJob", msjob)
	}

	services, _ := r.GetServicesForJob(job)
	if len(services) > 0 {
		msSchedHostStr, msSchedPortStr := getServiceIpAndPort(services[0])
		msServerNumStr, msWorkerNumStr := genPSAndWorkerNum(msjob)
		if msSchedHostStr == "" || msSchedPortStr == "" {
			return fmt.Errorf("%v scheduler service has no hostip or port, which is %s:%s", msjob, msSchedHostStr, msSchedPortStr)
		}
		// // Add environment variable to ms container in the pod.
		for i := range podTemplate.Spec.Containers {
			if podTemplate.Spec.Containers[i].Name == mindsporev1.DefaultContainerName {
				if len(podTemplate.Spec.Containers[i].Env) == 0 {
					podTemplate.Spec.Containers[i].Env = make([]corev1.EnvVar, 0)
				}
				// Scheduler env of msSchedHost must be the podIP, and other roles's are the service IP of scheduler.
				if rtype == strings.ToLower(string(mindsporev1.MSReplicaTypeScheduler)) {
					podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
						Name: msSchedHost,
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					})
				} else {
					podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
						Name:  msSchedHost,
						Value: msSchedHostStr,
					})
				}
				podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
					Name: msNodeId,
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				})
				podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  msSchedPort,
					Value: msSchedPortStr,
				})
				podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  msServerNum,
					Value: msServerNumStr,
				})
				podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  msWorkerNum,
					Value: msWorkerNumStr,
				})
				podTemplate.Spec.Containers[i].Env = append(podTemplate.Spec.Containers[i].Env, corev1.EnvVar{
					Name:  msRole,
					Value: msRoleMap[rtype],
				})
			}
		}
	}
	return nil
}

func (r *MSJobReconciler) GetDefaultContainerName() string {
	return mindsporev1.DefaultContainerName
}

func (r *MSJobReconciler) GetDefaultContainerPortName() string {
	return mindsporev1.DefaultPortName
}

func (r *MSJobReconciler) IsMasterRole(replicas map[commonv1.ReplicaType]*commonv1.ReplicaSpec,
	rtype commonv1.ReplicaType, index int) bool {
	return false
}

// IsWorker0Completed returns true if pod of worker0 succeeded and exited with 0
func (r *MSJobReconciler) IsWorker0Completed(msjob *mindsporev1.MSJob, replicas map[commonv1.ReplicaType]*commonv1.ReplicaSpec) (bool, error) {
	worker0Completed := false
	_, ok := replicas[mindsporev1.MSReplicaTypeWorker]
	if !ok {
		return true, nil
	}
	podSlices, err := r.getPodSlices(msjob, replicas[mindsporev1.MSReplicaTypeWorker].Replicas)
	if err != nil {
		return false, err
	}
	for index, podSlice := range podSlices {
		if len(podSlice) == 1 {
			pod := podSlice[0]
			exitCode := getContainerExitCode(pod)
			if index == 0 && exitCode == 0 && pod.Status.Phase == v1.PodSucceeded {
				worker0Completed = true
			}
		}
	}
	return worker0Completed, nil
}

// getPodSlices returns a slice, which element is the slice of pod.
// It gives enough information to caller to make decision to up/down scale resources.
func (r *MSJobReconciler) getPodSlices(msjob *mindsporev1.MSJob, replicasNum *int32) ([][]*v1.Pod, error) {
	logger := commonutil.LoggerForReplica(msjob, strings.ToLower(string(mindsporev1.MSReplicaTypeWorker)))

	pods, err := r.GetPodsForJob(msjob)
	if err != nil {
		commonutil.LoggerForJob(msjob).Warnf("getPodsForMSJob error %v", err)
		return nil, err
	}

	// Get all pods for the type rt.
	pods, err = r.JobController.FilterPodsForReplicaType(pods, strings.ToLower(string(mindsporev1.MSReplicaTypeWorker)))
	if err != nil {
		return nil, err
	}

	podSlices := r.GetPodSlices(pods, int(*replicasNum), logger)
	return podSlices, nil
}

// reconcilePods checks and updates pods for each given MSReplicaSpec.
// It will requeue the msjob in case of an error while creating/deleting pods.
func (r *MSJobReconciler) ReconcilePods(
	job interface{},
	jobStatus *commonv1.JobStatus,
	pods []*v1.Pod,
	rtype commonv1.ReplicaType,
	spec *commonv1.ReplicaSpec,
	replicas map[commonv1.ReplicaType]*commonv1.ReplicaSpec,
) error {

	msJob, ok := job.(*mindsporev1.MSJob)
	if !ok {
		return fmt.Errorf("%v is not a type of MSJob", msJob)
	}

	// Convert ReplicaType to lower string.
	rt := strings.ToLower(string(rtype))
	logger := commonutil.LoggerForJob(msJob)
	// Get all pods for the type rt.
	pods, err := r.FilterPodsForReplicaType(pods, rt)
	if err != nil {
		return err
	}
	numReplicas := int(*spec.Replicas)
	initializeReplicaStatuses(jobStatus, rtype)

	// GetPodSlices will return enough information here to make decision to add/remove/update resources.
	//
	// For example, let's assume we have pods with replica-index 0, 1, 2
	// If replica is 4, return a slice with size 4. [[0],[1],[2],[]], a pod with replica-index 3 will be created.
	//
	// If replica is 1, return a slice with size 3. [[0],[1],[2]], pod with replica-index 1 and 2 are out of range and will be deleted.
	podSlices := r.GetPodSlices(pods, numReplicas, logger)
	for index, podSlice := range podSlices {
		if len(podSlice) > 1 {
			logger.Warningf("We have too many pods for %s %d", rt, index)
		} else if len(podSlice) == 0 {
			logger.Infof("Need to create new pod: %s-%d", rt, index)

			// check if this replica is the master role
			// TODO: [should change to CreateNewPod]
			err = r.createNewPod(msJob, rt, strconv.Itoa(index), spec, replicas)
			if err != nil {
				return err
			}
		} else {
			// Check the status of the current pod.
			pod := podSlice[0]

			// check if the index is in the valid range, if not, we should kill the pod
			if index < 0 || index >= numReplicas {
				err = r.PodControl.DeletePod(pod.Namespace, pod.Name, msJob)
				if err != nil {
					return err
				}
			}
			// Get the exit code of the container.
			var exitCode int32 = 0xbeef // magic number
			for _, status := range pod.Status.ContainerStatuses {
				state := status.State
				if status.Name == r.GetDefaultContainerName() && state.Terminated != nil {
					exitCode = state.Terminated.ExitCode
					logger.Infof("Pod: %v.%v exited with code %v", pod.Namespace, pod.Name, exitCode)
					r.Recorder.Eventf(msJob, v1.EventTypeNormal, exitedWithCodeReason, "Pod: %v.%v exited with code %v", pod.Namespace, pod.Name, exitCode)
				}
			}
			// Check if the pod is retryable.
			if spec.RestartPolicy == commonv1.RestartPolicyExitCode {
				if pod.Status.Phase == v1.PodFailed && train_util.IsRetryableExitCode(exitCode) {
					logger.Infof("Need to restart the pod: %v.%v", pod.Namespace, pod.Name)
					if err := r.PodControl.DeletePod(pod.Namespace, pod.Name, msJob); err != nil {
						return err
					}

					// with common library framework, we have to handle restart status here
					// or we won't know which replica has been restarted in updateJobStatus after reconciling all replicas
					msg := fmt.Sprintf("MSJob %s is restarting because %s replica(s) failed.",
						msJob.Name, rtype)
					r.Recorder.Event(msJob, corev1.EventTypeWarning, msJobRestartingReason, msg)
					err := commonutil.UpdateJobConditions(jobStatus, commonv1.JobRestarting, msJobRestartingReason, msg)
					if err != nil {
						commonutil.LoggerForJob(msJob).Infof("Append msjob condition error: %v", err)
						return err
					}
					trainingoperatorcommon.RestartedJobsCounterInc(msJob.Namespace, mindsporev1.FrameworkName)
				}
			}

			updateJobReplicaStatuses(jobStatus, rtype, pod)
		}
	}
	return nil
}

// createNewPod creates a new pod for the given index and type.
func (r *MSJobReconciler) createNewPod(msjob *mindsporev1.MSJob, rt, index string, spec *commonv1.ReplicaSpec,
	replicas map[commonv1.ReplicaType]*commonv1.ReplicaSpec) error {

	msjobKey, err := common.KeyFunc(msjob)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for msjob object %#v: %v", msjob, err))
		return err
	}
	expectationPodsKey := expectation.GenExpectationPodsKey(msjobKey, rt)
	err = r.Expectations.ExpectCreations(expectationPodsKey, 1)
	if err != nil {
		return err
	}
	logger := commonutil.LoggerForReplica(msjob, rt)
	// Create OwnerReference.
	controllerRef := r.GenOwnerReference(msjob)

	// Set type and index for the worker.
	labels := r.GenLabels(msjob.Name)
	labels[msReplicaTypeLabel] = rt
	labels[msReplicaIndexLabel] = index
	labels[commonv1.ReplicaTypeLabel] = rt
	labels[commonv1.ReplicaIndexLabel] = index

	podTemplate := spec.Template.DeepCopy()

	// Set name for the template.
	podTemplate.Name = common.GenGeneralName(msjob.Name, rt, index)

	if podTemplate.Labels == nil {
		podTemplate.Labels = make(map[string]string)
	}

	for key, value := range labels {
		podTemplate.Labels[key] = value
	}

	if err := r.SetClusterSpec(msjob, podTemplate, rt, index); err != nil {
		return err
	}

	// Submit a warning event if the user specifies restart policy for
	// the pod template. We recommend to set it from the replica level.
	if podTemplate.Spec.RestartPolicy != v1.RestartPolicy("") {
		errMsg := "Restart policy in pod template will be overwritten by restart policy in replica spec"
		logger.Warning(errMsg)
		r.Recorder.Event(msjob, v1.EventTypeWarning, podTemplateRestartPolicyReason, errMsg)
	}
	setRestartPolicy(podTemplate, spec)

	// if gang-scheduling is enabled:
	// 1. if user has specified other scheduler, we report a warning without overriding any fields.
	// 2. if no SchedulerName is set for pods, then we set the SchedulerName to "volcano".
	if r.Config.EnableGangScheduling {
		podSchedulerName := util.GetSchedulerName(replicas)
		if len(podSchedulerName) == 0 {
			podTemplate.Spec.SchedulerName = gangSchedulerName
		} else if strings.Compare(podSchedulerName, gangSchedulerName) != 0 {
			errMsg := "Another scheduler is specified when gang-scheduling is enabled and it will not be overwritten"
			logger.Warning(errMsg)
			r.Recorder.Event(msjob, v1.EventTypeWarning, podTemplateSchedulerNameReason, errMsg)
		}

		if podTemplate.Annotations == nil {
			podTemplate.Annotations = map[string]string{}
		}
		podTemplate.Annotations[gangSchedulingPodGroupAnnotation] = msjob.GetName()
		podTemplate.Annotations[volcanoTaskSpecKey] = rt
	}

	err = r.PodControl.CreatePodsWithControllerRef(msjob.Namespace, podTemplate, msjob, controllerRef)
	if err != nil && errors.IsTimeout(err) {
		// Pod is created but its initialization has timed out.
		// If the initialization is successful eventually, the
		// controller will observe the creation via the informer.
		// If the initialization fails, or if the pod keeps
		// uninitialized for a long time, the informer will not
		// receive any update, and the controller will create a new
		// pod when the expectation expires.
		return nil
	} else if err != nil {
		// Decrement the expected number of creates because the informer won't observe this pod
		logger.Infof(
			"Failed creation, decrementing expectations for msjob %s/%s, key %s",
			msjob.Namespace, msjob.Name, expectationPodsKey)
		r.Expectations.CreationObserved(expectationPodsKey)
		return err
	}
	return nil
}

// reconcileServices checks and updates services for each given ReplicaSpec.
// It will requeue the job in case of an error while creating/deleting services.
func (jc *MSJobReconciler) ReconcileServices(
	job metav1.Object,
	services []*v1.Service,
	rtype commonv1.ReplicaType,
	spec *commonv1.ReplicaSpec) error {

	if rtype != mindsporev1.MSReplicaTypeScheduler {
		return nil
	}
	// Convert ReplicaType to lower string.
	rt := strings.ToLower(string(rtype))
	replicas := int(*spec.Replicas)
	// Get all services for the type rt.
	services, err := jc.FilterServicesForReplicaType(services, rt)
	if err != nil {
		return err
	}

	// GetServiceSlices will return enough information here to make decision to add/remove/update resources.
	//
	// For example, let's assume we have services with replica-index 0, 1, 2
	// If replica is 4, return a slice with size 4. [[0],[1],[2],[]], a svc with replica-index 3 will be created.
	//
	// If replica is 1, return a slice with size 3. [[0],[1],[2]], svc with replica-index 1 and 2 are out of range and will be deleted.
	serviceSlices := jc.GetServiceSlices(services, replicas, commonutil.LoggerForReplica(job, rt))

	for index, serviceSlice := range serviceSlices {
		if len(serviceSlice) > 1 {
			commonutil.LoggerForReplica(job, rt).Warningf("We have too many services for %s %d", rtype, index)
		} else if len(serviceSlice) == 0 {
			commonutil.LoggerForReplica(job, rt).Infof("need to create new service: %s-%d", rtype, index)
			err = jc.CreateNewService(job, rtype, spec, strconv.Itoa(index))
			if err != nil {
				return err
			}
		} else {
			// Check the status of the current svc.
			svc := serviceSlice[0]

			// check if the index is in the valid range, if not, we should kill the svc
			if index < 0 || index >= replicas {
				err = jc.ServiceControl.DeleteService(svc.Namespace, svc.Name, job.(runtime.Object))
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// createNewService creates a new service for the given index and type.
func (jc *MSJobReconciler) CreateNewService(job metav1.Object, rtype commonv1.ReplicaType,
	spec *commonv1.ReplicaSpec, index string) error {
	jobKey, err := common.KeyFunc(job)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for job object %#v: %v", job, err))
		return err
	}

	rt := strings.ToLower(string(rtype))
	// Append ReplicaTypeLabelDeprecated and ReplicaIndexLabelDeprecated labels.
	labels := jc.GenLabels(job.GetName())
	utillabels.SetReplicaType(labels, rt)
	utillabels.SetReplicaIndexStr(labels, index)

	ports, err := jc.GetPortsFromJob(spec)
	if err != nil {
		return err
	}

	service := &v1.Service{
		Spec: v1.ServiceSpec{
			Selector: labels,
			Ports:    []v1.ServicePort{},
		},
	}

	// Add service ports
	for name, port := range ports {
		svcPort := v1.ServicePort{Name: name, Port: port}
		service.Spec.Ports = append(service.Spec.Ports, svcPort)
	}

	service.Name = common.GenGeneralName(job.GetName(), rt, index)
	service.Labels = labels
	// Create OwnerReference.
	controllerRef := jc.GenOwnerReference(job)

	// Creation is expected when there is no error returned
	expectationServicesKey := expectation.GenExpectationServicesKey(jobKey, rt)
	jc.Expectations.RaiseExpectations(expectationServicesKey, 1, 0)

	err = jc.ServiceControl.CreateServicesWithControllerRef(job.GetNamespace(), service, job.(runtime.Object), controllerRef)
	if err != nil && errors.IsTimeout(err) {
		// Service is created but its initialization has timed out.
		// If the initialization is successful eventually, the
		// controller will observe the creation via the informer.
		// If the initialization fails, or if the service keeps
		// uninitialized for a long time, the informer will not
		// receive any update, and the controller will create a new
		// service when the expectation expires.
		succeededServiceCreationCount.Inc()
		return nil
	} else if err != nil {
		// Since error occurred(the informer won't observe this service),
		// we decrement the expected number of creates
		// and wait until next reconciliation
		jc.Expectations.CreationObserved(expectationServicesKey)
		failedServiceCreationCount.Inc()
		return err
	}
	succeededServiceCreationCount.Inc()
	return nil
}

// onOwnerCreateFunc modify creation condition.
func (r *MSJobReconciler) onOwnerCreateFunc() func(event.CreateEvent) bool {
	return func(e event.CreateEvent) bool {
		msJob, ok := e.Object.(*mindsporev1.MSJob)
		if !ok {
			return true
		}

		r.Scheme.Default(msJob)
		msg := fmt.Sprintf("MSJob %s is created.", e.Object.GetName())
		logrus.Info(msg)
		trainingoperatorcommon.CreatedJobsCounterInc(msJob.Namespace, mindsporev1.FrameworkName)
		if err := commonutil.UpdateJobConditions(&msJob.Status, commonv1.JobCreated, "MSJobCreated", msg); err != nil {
			log.Log.Error(err, "append job condition error")
			return false
		}
		return true
	}
}
