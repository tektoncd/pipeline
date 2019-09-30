package v1alpha1

import (
	"fmt"
	"github.com/tektoncd/pipeline/pkg/reconciler"
	"time"

	clientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubernetes/pkg/util/metrics"
	"knative.dev/pkg/logging/logkey"
)

const ExpirationTTL = "expiration-ttl-controller"

type Options struct {
	kubernetes kubernetes.Interface
	clientset  clientset.Interface
	recorder   record.EventRecorder
	Logger     *zap.SugaredLogger
}

type ExpirationController struct {
	client   clientset.Interface
	recorder record.EventRecorder

	// trLister can list/get TaskRuns from the shared informer's store
	trLister listers.TaskRunLister
	prLister listers.PipelineRunLister

	// jStoreSynced returns true if the TaskRun store has been synced at least once.
	// Added as a member to the struct to allow injection for testing.
	ListerSynced cache.InformerSynced

	// TaskRuns that the controller will check its TTL and attempt to delete when the TTL expires.
	queue workqueue.RateLimitingInterface

	// The clock for tracking time
	clock clock.Clock
}

// New creates an instance of Controller
//func NewTaskRun(taskRunInformer externalInformers.TaskRunInformer, opt Options) *ExpirationController {
//	// Enrich the logs with controller name
//	logger := opt.Logger.Named(ExpirationTTL).With(zap.String(logkey.ControllerType, ExpirationTTL))
//
//	// Use recorder provided in options if presents.   Otherwise, create a new one.
//	recorder := opt.recorder
//
//	if recorder == nil {
//		eventBroadcaster := record.NewBroadcaster()
//		eventBroadcaster.StartLogging(klog.Infof)
//		eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: opt.kubernetes.CoreV1().Events("")})
//		recorder = eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: ExpirationTTL})
//	} else {
//		logger.Debug("Using recorder from option")
//	}
//
//	if opt.clientset != nil && opt.clientset.TektonV1alpha1().RESTClient().GetRateLimiter() != nil {
//		metrics.RegisterMetricAndTrackRateLimiterUsage(ExpirationTTL, opt.clientset.TektonV1alpha1().RESTClient().GetRateLimiter())
//	}
//
//	tc := &ExpirationController{
//		client:   opt.clientset,
//		recorder: recorder,
//		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ttl_taskruns_to_delete"),
//	}
//
//	taskRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
//		AddFunc:    tc.addTaskRun,
//		UpdateFunc: tc.updateTaskRun,
//	})
//
//	tc.trLister = taskRunInformer.Lister()
//	tc.ListerSynced = taskRunInformer.Informer().HasSynced
//
//	tc.clock = clock.RealClock{}
//
//	return tc
//}

// New creates an instance of Controller
func NewExpirationController(opt reconciler.Options, controllerAgentName string) *ExpirationController {
	// Enrich the logs with controller name
	logger := opt.Logger.Named(controllerAgentName).With(zap.String(logkey.ControllerType, controllerAgentName))

	// Use recorder provided in options if presents.   Otherwise, create a new one.
	recorder := opt.Recorder

	if recorder == nil {
		eventBroadcaster := record.NewBroadcaster()
		eventBroadcaster.StartLogging(klog.Infof)
		eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: opt.KubeClientSet.CoreV1().Events("")})
		recorder = eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: controllerAgentName})
	} else {
		logger.Debug("Using recorder from option")
	}

	if opt.PipelineClientSet != nil && opt.PipelineClientSet.TektonV1alpha1().RESTClient().GetRateLimiter() != nil {
		metrics.RegisterMetricAndTrackRateLimiterUsage(controllerAgentName, opt.PipelineClientSet.TektonV1alpha1().RESTClient().GetRateLimiter())
	}

	tc := &ExpirationController{
		client:   opt.PipelineClientSet,
		recorder: recorder,
		queue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ttl_taskruns_to_delete"),
	}

	//pipelineRunInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
	//	AddFunc:    tc.addPipelineRun,
	//	UpdateFunc: tc.updatePipelineRun,
	//})
	//
	//tc.prLister = pipelineRunInformer.Lister()
	//tc.ListerSynced = pipelineRunInformer.Informer().HasSynced

	tc.clock = clock.RealClock{}

	return tc
}

// Run starts the workers to clean up TaskRuns.
func (tc *ExpirationController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer tc.queue.ShutDown()

	klog.Infof("Starting TTL after succeeded controller")
	defer klog.Infof("Shutting down TTL after succeeded controller")

	if !cache.WaitForNamedCacheSync("TTL after succeeded", stopCh, tc.ListerSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(tc.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (tc *ExpirationController) worker() {
	for tc.processNextWorkItem() {
	}
}

func (tc *ExpirationController) processNextWorkItem() bool {
	key, quit := tc.queue.Get()
	if quit {
		return false
	}
	defer tc.queue.Done(key)

	err := tc.processTaskRun(key.(string))
	tc.handleTrErr(err, key)

	err = tc.processPipelineRun(key.(string))
	tc.handlePrErr(err, key)

	return true
}

func (tc *ExpirationController) handleTrErr(err error, key interface{}) {
	if err == nil {
		tc.queue.Forget(key)
		return
	}

	utilruntime.HandleError(fmt.Errorf("error cleaning up TaskRun %v, will retry: %v", key, err))
	tc.queue.AddRateLimited(key)
}

func (tc *ExpirationController) handlePrErr(err error, key interface{}) {
	if err == nil {
		tc.queue.Forget(key)
		return
	}

	utilruntime.HandleError(fmt.Errorf("error cleaning up PipelineRun %v, will retry: %v", key, err))
	tc.queue.AddRateLimited(key)
}

//type ControllerContext struct {
//	informers.SharedInformerFactory
//	Stop <-chan struct{}
//}
//
//func startTTLAfterFinishedController(ctx ControllerContext) (http.Handler, bool, error) {
//	if !utilfeature.DefaultFeatureGate.Enabled(features.TTLAfterFinished) {
//		return nil, false, nil
//	}
//	opt := reconciler.Options{
//		KubeClientSet:     kubeclientset,
//		PipelineClientSet: pipelineclientset,
//		ConfigMapWatcher:  cmw,
//		ResyncPeriod:      resyncPeriod,
//		Logger:            logger,
//	}
//	go New(
//		ctx.Tekton().V1alpha1().TaskRuns(),
//		ctx.Tekton().V1alpha1().PipelineRuns(),
//		ctx..ClientOrDie("ttl-after-finished-controller"),
//	).Run(int(ctx.ComponentConfig.TTLAfterFinishedController.ConcurrentTTLSyncs), ctx.Stop)
//	return nil, true, nil
//}