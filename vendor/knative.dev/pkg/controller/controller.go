/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"knative.dev/pkg/kmeta"
	kle "knative.dev/pkg/leaderelection"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/logging/logkey"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"
)

const (
	falseString = "false"
	trueString  = "true"

	// DefaultResyncPeriod is the default duration that is used when no
	// resync period is associated with a controllers initialization context.
	DefaultResyncPeriod = 10 * time.Hour
)

var (
	// DefaultThreadsPerController is the number of threads to use
	// when processing the controller's workqueue.  Controller binaries
	// may adjust this process-wide default.  For finer control, invoke
	// Run on the controller directly.
	// TODO rename the const to Concurrency and deprecated this
	DefaultThreadsPerController = 2
)

// Reconciler is the interface that controller implementations are expected
// to implement, so that the shared controller.Impl can drive work through it.
type Reconciler interface {
	Reconcile(ctx context.Context, key string) error
}

// PassNew makes it simple to create an UpdateFunc for use with
// cache.ResourceEventHandlerFuncs that can delegate the same methods
// as AddFunc/DeleteFunc but passing through only the second argument
// (which is the "new" object).
func PassNew(f func(interface{})) func(interface{}, interface{}) {
	return func(first, second interface{}) {
		f(second)
	}
}

// HandleAll wraps the provided handler function into a cache.ResourceEventHandler
// that sends all events to the given handler.  For Updates, only the new object
// is forwarded.
func HandleAll(h func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    h,
		UpdateFunc: PassNew(h),
		DeleteFunc: h,
	}
}

// Filter makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on the
// schema.GroupVersionKind of the controlling resources.
//
// Deprecated: Use FilterGroupVersionKind or FilterGroupKind instead
func Filter(gvk schema.GroupVersionKind) func(obj interface{}) bool {
	return FilterGroupVersionKind(gvk)
}

// FilterGroupVersionKind makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on the
// schema.GroupVersionKind of the controlling resources.
//
// Deprecated: Use FilterControllerGVK instead.
func FilterGroupVersionKind(gvk schema.GroupVersionKind) func(obj interface{}) bool {
	return FilterControllerGVK(gvk)
}

// FilterControllerGVK makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on the
// schema.GroupVersionKind of the controlling resources.
func FilterControllerGVK(gvk schema.GroupVersionKind) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		object, ok := obj.(metav1.Object)
		if !ok {
			return false
		}

		owner := metav1.GetControllerOf(object)
		return owner != nil &&
			owner.APIVersion == gvk.GroupVersion().String() &&
			owner.Kind == gvk.Kind
	}
}

// FilterGroupKind makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on the
// schema.GroupKind of the controlling resources.
//
// Deprecated: Use FilterControllerGK instead
func FilterGroupKind(gk schema.GroupKind) func(obj interface{}) bool {
	return FilterControllerGK(gk)
}

// FilterControllerGK makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on the
// schema.GroupKind of the controlling resources.
func FilterControllerGK(gk schema.GroupKind) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		object, ok := obj.(metav1.Object)
		if !ok {
			return false
		}

		owner := metav1.GetControllerOf(object)
		if owner == nil {
			return false
		}

		ownerGV, err := schema.ParseGroupVersion(owner.APIVersion)
		return err == nil &&
			ownerGV.Group == gk.Group &&
			owner.Kind == gk.Kind
	}
}

// FilterController makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on the
// controlling resource.
func FilterController(r kmeta.OwnerRefable) func(obj interface{}) bool {
	return FilterControllerGK(r.GetGroupVersionKind().GroupKind())
}

// FilterWithName makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on a name.
func FilterWithName(name string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if object, ok := obj.(metav1.Object); ok {
			return name == object.GetName()
		}
		return false
	}
}

// FilterWithNameAndNamespace makes it simple to create FilterFunc's for use with
// cache.FilteringResourceEventHandler that filter based on a namespace and a name.
func FilterWithNameAndNamespace(namespace, name string) func(obj interface{}) bool {
	return func(obj interface{}) bool {
		if object, ok := obj.(metav1.Object); ok {
			return name == object.GetName() &&
				namespace == object.GetNamespace()
		}
		return false
	}
}

// Impl is our core controller implementation.  It handles queuing and feeding work
// from the queue to an implementation of Reconciler.
type Impl struct {
	// Name is the unique name for this controller workqueue within this process.
	// This is used for surfacing metrics, and per-controller leader election.
	Name string

	// Reconciler is the workhorse of this controller, it is fed the keys
	// from the workqueue to process.  Public for testing.
	Reconciler Reconciler

	// workQueue is a rate-limited two-lane work queue.
	// This is used to queue work to be processed instead of performing it as
	// soon as a change happens. This means we can ensure we only process a
	// fixed amount of resources at a time, and makes it easy to ensure we are
	// never processing the same item simultaneously in two different workers.
	// The slow queue is used for global resync and other background processes
	// which are not required to complete at the highest priority.
	workQueue *twoLaneQueue

	// Concurrency - The number of workers to use when processing the controller's workqueue.
	Concurrency int

	// Sugared logger is easier to use but is not as performant as the
	// raw logger. In performance critical paths, call logger.Desugar()
	// and use the returned raw logger instead. In addition to the
	// performance benefits, raw logger also preserves type-safety at
	// the expense of slightly greater verbosity.
	logger *zap.SugaredLogger

	// StatsReporter is used to send common controller metrics.
	statsReporter StatsReporter

	// Tracker allows reconcilers to associate a reference with particular key,
	// such that when the reference changes the key is queued for reconciliation.
	Tracker tracker.Interface
}

// ControllerOptions encapsulates options for creating a new controller,
// including throttling and stats behavior.
type ControllerOptions struct { //nolint // for backcompat.
	WorkQueueName string
	Logger        *zap.SugaredLogger
	Reporter      StatsReporter
	RateLimiter   workqueue.RateLimiter
	Concurrency   int
}

// NewImpl instantiates an instance of our controller that will feed work to the
// provided Reconciler as it is enqueued.
// Deprecated: use NewImplFull.
func NewImpl(r Reconciler, logger *zap.SugaredLogger, workQueueName string) *Impl {
	return NewImplFull(r, ControllerOptions{WorkQueueName: workQueueName, Logger: logger})
}

// NewImplFull accepts the full set of options available to all controllers.
// Deprecated: use NewContext instead.
func NewImplFull(r Reconciler, options ControllerOptions) *Impl {
	return NewContext(context.TODO(), r, options)
}

// NewContext instantiates an instance of our controller that will feed work to the
// provided Reconciler as it is enqueued.
func NewContext(ctx context.Context, r Reconciler, options ControllerOptions) *Impl {
	if options.RateLimiter == nil {
		options.RateLimiter = workqueue.DefaultControllerRateLimiter()
	}
	if options.Reporter == nil {
		options.Reporter = MustNewStatsReporter(options.WorkQueueName, options.Logger)
	}
	if options.Concurrency == 0 {
		options.Concurrency = DefaultThreadsPerController
	}
	i := &Impl{
		Name:          options.WorkQueueName,
		Reconciler:    r,
		workQueue:     newTwoLaneWorkQueue(options.WorkQueueName, options.RateLimiter),
		logger:        options.Logger,
		statsReporter: options.Reporter,
		Concurrency:   options.Concurrency,
	}

	if t := GetTracker(ctx); t != nil {
		i.Tracker = t
	} else {
		i.Tracker = tracker.New(i.EnqueueKey, GetTrackerLease(ctx))
	}

	return i
}

// WorkQueue permits direct access to the work queue.
func (c *Impl) WorkQueue() workqueue.RateLimitingInterface {
	return c.workQueue
}

// EnqueueAfter takes a resource, converts it into a namespace/name string,
// and passes it to EnqueueKey.
func (c *Impl) EnqueueAfter(obj interface{}, after time.Duration) {
	object, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		c.logger.Errorw("Enqueue", zap.Error(err))
		return
	}
	c.EnqueueKeyAfter(types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()}, after)
}

// EnqueueSlowKey takes a resource, converts it into a namespace/name string,
// and enqueues that key in the slow lane.
func (c *Impl) EnqueueSlowKey(key types.NamespacedName) {
	c.workQueue.SlowLane().Add(key)

	if logger := c.logger.Desugar(); logger.Core().Enabled(zapcore.DebugLevel) {
		logger.Debug(fmt.Sprintf("Adding to the slow queue %s (depth(total/slow): %d/%d)",
			safeKey(key), c.workQueue.Len(), c.workQueue.SlowLane().Len()),
			zap.String(logkey.Key, key.String()))
	}
}

// EnqueueSlow extracts namespaced name from the object and enqueues it on the slow
// work queue.
func (c *Impl) EnqueueSlow(obj interface{}) {
	object, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		c.logger.Errorw("EnqueueSlow", zap.Error(err))
		return
	}
	key := types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()}
	c.EnqueueSlowKey(key)
}

// Enqueue takes a resource, converts it into a namespace/name string,
// and passes it to EnqueueKey.
func (c *Impl) Enqueue(obj interface{}) {
	object, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		c.logger.Errorw("Enqueue", zap.Error(err))
		return
	}
	c.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: object.GetName()})
}

// EnqueueSentinel returns a Enqueue method which will always enqueue a
// predefined key instead of the object key.
func (c *Impl) EnqueueSentinel(k types.NamespacedName) func(interface{}) {
	return func(interface{}) {
		c.EnqueueKey(k)
	}
}

// EnqueueControllerOf takes a resource, identifies its controller resource,
// converts it into a namespace/name string, and passes that to EnqueueKey.
func (c *Impl) EnqueueControllerOf(obj interface{}) {
	object, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		c.logger.Error(err)
		return
	}

	// If we can determine the controller ref of this object, then
	// add that object to our workqueue.
	if owner := metav1.GetControllerOf(object); owner != nil {
		c.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: owner.Name})
	}
}

// EnqueueLabelOfNamespaceScopedResource returns with an Enqueue func that
// takes a resource, identifies its controller resource through given namespace
// and name labels, converts it into a namespace/name string, and passes that
// to EnqueueKey. The controller resource must be of namespace-scoped.
func (c *Impl) EnqueueLabelOfNamespaceScopedResource(namespaceLabel, nameLabel string) func(obj interface{}) {
	return func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			c.logger.Error(err)
			return
		}

		labels := object.GetLabels()
		controllerKey, ok := labels[nameLabel]
		if !ok {
			c.logger.Debugf("Object %s/%s does not have a referring name label %s",
				object.GetNamespace(), object.GetName(), nameLabel)
			return
		}

		if namespaceLabel != "" {
			controllerNamespace, ok := labels[namespaceLabel]
			if !ok {
				c.logger.Debugf("Object %s/%s does not have a referring namespace label %s",
					object.GetNamespace(), object.GetName(), namespaceLabel)
				return
			}

			c.EnqueueKey(types.NamespacedName{Namespace: controllerNamespace, Name: controllerKey})
			return
		}

		// Pass through namespace of the object itself if no namespace label specified.
		// This is for the scenario that object and the parent resource are of same namespace,
		// e.g. to enqueue the revision of an endpoint.
		c.EnqueueKey(types.NamespacedName{Namespace: object.GetNamespace(), Name: controllerKey})
	}
}

// EnqueueLabelOfClusterScopedResource returns with an Enqueue func
// that takes a resource, identifies its controller resource through
// given name label, and passes it to EnqueueKey.
// The controller resource must be of cluster-scoped.
func (c *Impl) EnqueueLabelOfClusterScopedResource(nameLabel string) func(obj interface{}) {
	return func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err != nil {
			c.logger.Error(err)
			return
		}

		labels := object.GetLabels()
		controllerKey, ok := labels[nameLabel]
		if !ok {
			c.logger.Debugf("Object %s/%s does not have a referring name label %s",
				object.GetNamespace(), object.GetName(), nameLabel)
			return
		}

		c.EnqueueKey(types.NamespacedName{Namespace: "", Name: controllerKey})
	}
}

// EnqueueNamespaceOf takes a resource, and enqueues the Namespace to which it belongs.
func (c *Impl) EnqueueNamespaceOf(obj interface{}) {
	object, err := kmeta.DeletionHandlingAccessor(obj)
	if err != nil {
		c.logger.Errorw("EnqueueNamespaceOf", zap.Error(err))
		return
	}
	c.EnqueueKey(types.NamespacedName{Name: object.GetNamespace()})
}

// EnqueueKey takes a namespace/name string and puts it onto the work queue.
func (c *Impl) EnqueueKey(key types.NamespacedName) {
	c.workQueue.Add(key)

	if logger := c.logger.Desugar(); logger.Core().Enabled(zapcore.DebugLevel) {
		logger.Debug(fmt.Sprintf("Adding to queue %s (depth: %d)", safeKey(key), c.workQueue.Len()),
			zap.String(logkey.Key, key.String()))
	}
}

// MaybeEnqueueBucketKey takes a Bucket and namespace/name string and puts it onto
// the slow work queue.
func (c *Impl) MaybeEnqueueBucketKey(bkt reconciler.Bucket, key types.NamespacedName) {
	if bkt.Has(key) {
		c.EnqueueSlowKey(key)
	}
}

// EnqueueKeyAfter takes a namespace/name string and schedules its execution in
// the work queue after given delay.
func (c *Impl) EnqueueKeyAfter(key types.NamespacedName, delay time.Duration) {
	c.workQueue.AddAfter(key, delay)

	if logger := c.logger.Desugar(); logger.Core().Enabled(zapcore.DebugLevel) {
		logger.Debug(fmt.Sprintf("Adding to queue %s (delay: %v, depth: %d)", safeKey(key), delay, c.workQueue.Len()),
			zap.String(logkey.Key, key.String()))
	}
}

// RunContext starts the controller's worker threads, the number of which is threadiness.
// If the context has been decorated for LeaderElection, then an elector is built and run.
// It then blocks until the context is cancelled, at which point it shuts down its
// internal work queue and waits for workers to finish processing their current
// work items.
func (c *Impl) RunContext(ctx context.Context, threadiness int) error {
	sg := sync.WaitGroup{}
	defer func() {
		c.workQueue.ShutDown()
		for c.workQueue.Len() > 0 {
			time.Sleep(time.Millisecond * 100)
		}
		sg.Wait()
		runtime.HandleCrash()
	}()

	if la, ok := c.Reconciler.(reconciler.LeaderAware); ok {
		// Build and execute an elector.
		le, err := kle.BuildElector(ctx, la, c.Name, c.MaybeEnqueueBucketKey)
		if err != nil {
			return err
		}
		sg.Add(1)
		go func() {
			defer sg.Done()
			le.Run(ctx)
		}()
	}

	// Launch workers to process resources that get enqueued to our workqueue.
	c.logger.Info("Starting controller and workers")
	for i := 0; i < threadiness; i++ {
		sg.Add(1)
		go func() {
			defer sg.Done()
			for c.processNextWorkItem() {
			}
		}()
	}

	c.logger.Info("Started workers")
	<-ctx.Done()
	c.logger.Info("Shutting down workers")

	return nil
}

// Run runs the controller.
//
// Deprecated: Use RunContext instead.
func (c *Impl) Run(threadiness int, stopCh <-chan struct{}) error {
	// Create a context that is cancelled when the stopCh is called.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
	}()
	return c.RunContext(ctx, threadiness)
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling Reconcile on our Reconciler.
func (c *Impl) processNextWorkItem() bool {
	obj, shutdown := c.workQueue.Get()
	if shutdown {
		return false
	}
	key := obj.(types.NamespacedName)
	keyStr := safeKey(key)

	c.logger.Debugf("Processing from queue %s (depth: %d)", safeKey(key), c.workQueue.Len())

	startTime := time.Now()
	// Send the metrics for the current queue depth
	c.statsReporter.ReportQueueDepth(int64(c.workQueue.Len()))

	var err error
	defer func() {
		status := trueString
		if err != nil {
			status = falseString
		}
		c.statsReporter.ReportReconcile(time.Since(startTime), status, key)

		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if
		// reconcile succeeds. If a transient error occurs, we do not call
		// Forget and put the item back to the queue with an increased
		// delay.
		c.workQueue.Done(key)
	}()

	// Embed the key into the logger and attach that to the context we pass
	// to the Reconciler.
	logger := c.logger.With(zap.String(logkey.TraceID, uuid.NewString()), zap.String(logkey.Key, keyStr))
	ctx := logging.WithLogger(context.Background(), logger)

	// Run Reconcile, passing it the namespace/name string of the
	// resource to be synced.
	if err = c.Reconciler.Reconcile(ctx, keyStr); err != nil {
		c.handleErr(err, key, startTime)
		return true
	}

	// Finally, if no error occurs we Forget this item so it does not
	// have any delay when another change happens.
	c.workQueue.Forget(key)
	logger.Infow("Reconcile succeeded", zap.Duration("duration", time.Since(startTime)))

	return true
}

func (c *Impl) handleErr(err error, key types.NamespacedName, startTime time.Time) {
	if IsSkipKey(err) {
		c.workQueue.Forget(key)
		return
	}
	if ok, delay := IsRequeueKey(err); ok {
		c.workQueue.AddAfter(key, delay)
		c.logger.Debugf("Requeuing key %s (by request) after %v (depth: %d)", safeKey(key), delay, c.workQueue.Len())
		return
	}

	c.logger.Errorw("Reconcile error", zap.Duration("duration", time.Since(startTime)), zap.Error(err))

	// Re-queue the key if it's a transient error.
	// We want to check that the queue is shutting down here
	// since controller Run might have exited by now (since while this item was
	// being processed, queue.Len==0).
	if !IsPermanentError(err) && !c.workQueue.ShuttingDown() {
		c.workQueue.AddRateLimited(key)
		c.logger.Debugf("Requeuing key %s due to non-permanent error (depth: %d)", safeKey(key), c.workQueue.Len())
		return
	}

	c.workQueue.Forget(key)
}

// GlobalResync enqueues into the slow lane all objects from the passed SharedInformer
func (c *Impl) GlobalResync(si cache.SharedInformer) {
	alwaysTrue := func(interface{}) bool { return true }
	c.FilteredGlobalResync(alwaysTrue, si)
}

// FilteredGlobalResync enqueues all objects from the
// SharedInformer that pass the filter function in to the slow queue.
func (c *Impl) FilteredGlobalResync(f func(interface{}) bool, si cache.SharedInformer) {
	if c.workQueue.ShuttingDown() {
		return
	}
	list := si.GetStore().List()
	for _, obj := range list {
		if f(obj) {
			c.EnqueueSlow(obj)
		}
	}
}

// NewSkipKey returns a new instance of skipKeyError.
// Users can return this type of error to indicate that the key was skipped.
func NewSkipKey(key string) error {
	return skipKeyError{key: key}
}

// skipKeyError is an error that indicates a key was skipped.
// We should not re-queue keys when it returns this error from Reconcile.
type skipKeyError struct {
	key string
}

var _ error = skipKeyError{}

// Error implements the Error() interface of error.
func (err skipKeyError) Error() string {
	return fmt.Sprintf("skipped key: %q", err.key)
}

// IsSkipKey returns true if the given error is a skipKeyError.
func IsSkipKey(err error) bool {
	return errors.Is(err, skipKeyError{})
}

// Is implements the Is() interface of error. It returns whether the target
// error can be treated as equivalent to a permanentError.
func (skipKeyError) Is(target error) bool {
	//nolint: errorlint // This check is actually fine.
	_, ok := target.(skipKeyError)
	return ok
}

// NewPermanentError returns a new instance of permanentError.
// Users can wrap an error as permanentError with this in reconcile
// when they do not expect the key to get re-queued.
func NewPermanentError(err error) error {
	return permanentError{e: err}
}

// permanentError is an error that is considered not transient.
// We should not re-queue keys when it returns with thus error in reconcile.
type permanentError struct {
	e error
}

// IsPermanentError returns true if the given error is a permanentError or
// wraps a permanentError.
func IsPermanentError(err error) bool {
	return errors.Is(err, permanentError{})
}

// Is implements the Is() interface of error. It returns whether the target
// error can be treated as equivalent to a permanentError.
func (permanentError) Is(target error) bool {
	//nolint: errorlint // This check is actually fine.
	_, ok := target.(permanentError)
	return ok
}

var _ error = permanentError{}

// Error implements the Error() interface of error.
func (err permanentError) Error() string {
	if err.e == nil {
		return ""
	}

	return err.e.Error()
}

// Unwrap implements the Unwrap() interface of error. It returns the error
// wrapped inside permanentError.
func (err permanentError) Unwrap() error {
	return err.e
}

// NewRequeueImmediately returns a new instance of requeueKeyError.
// Users can return this type of error to immediately requeue a key.
func NewRequeueImmediately() error {
	return requeueKeyError{}
}

// NewRequeueAfter returns a new instance of requeueKeyError.
// Users can return this type of error to requeue a key after a delay.
func NewRequeueAfter(dur time.Duration) error {
	return requeueKeyError{duration: dur}
}

// requeueKeyError is an error that indicates the reconciler wants to reprocess
// the key after a particular duration (possibly zero).
// We should re-queue keys with the desired duration when this is returned by Reconcile.
type requeueKeyError struct {
	duration time.Duration
}

var _ error = requeueKeyError{}

// Error implements the Error() interface of error.
func (err requeueKeyError) Error() string {
	return fmt.Sprintf("requeue after: %s", err.duration)
}

// IsRequeueKey returns true if the given error is a requeueKeyError.
func IsRequeueKey(err error) (bool, time.Duration) {
	rqe := requeueKeyError{}
	if errors.As(err, &rqe) {
		return true, rqe.duration
	}
	return false, 0
}

// Is implements the Is() interface of error. It returns whether the target
// error can be treated as equivalent to a requeueKeyError.
func (requeueKeyError) Is(target error) bool {
	//nolint: errorlint // This check is actually fine.
	_, ok := target.(requeueKeyError)
	return ok
}

// Informer is the group of methods that a type must implement to be passed to
// StartInformers.
type Informer interface {
	Run(<-chan struct{})
	HasSynced() bool
}

// StartInformers kicks off all of the passed informers and then waits for all
// of them to synchronize.
func StartInformers(stopCh <-chan struct{}, informers ...Informer) error {
	for _, informer := range informers {
		informer := informer
		go informer.Run(stopCh)
	}

	for i, informer := range informers {
		if ok := cache.WaitForCacheSync(stopCh, informer.HasSynced); !ok {
			return fmt.Errorf("failed to wait for cache at index %d to sync", i)
		}
	}
	return nil
}

// RunInformers kicks off all of the passed informers and then waits for all of
// them to synchronize. Returned function will wait for all informers to finish.
func RunInformers(stopCh <-chan struct{}, informers ...Informer) (func(), error) {
	var wg sync.WaitGroup
	wg.Add(len(informers))
	for _, informer := range informers {
		informer := informer
		go func() {
			defer wg.Done()
			informer.Run(stopCh)
		}()
	}

	for i, informer := range informers {
		if ok := WaitForCacheSyncQuick(stopCh, informer.HasSynced); !ok {
			return wg.Wait, fmt.Errorf("failed to wait for cache at index %d to sync", i)
		}
	}
	return wg.Wait, nil
}

// WaitForCacheSyncQuick is the same as cache.WaitForCacheSync but with a much reduced
// check-rate for the sync period.
func WaitForCacheSyncQuick(stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) bool {
	err := wait.PollImmediateUntil(time.Millisecond,
		func() (bool, error) {
			for _, syncFunc := range cacheSyncs {
				if !syncFunc() {
					return false, nil
				}
			}
			return true, nil
		},
		stopCh)
	return err == nil
}

// StartAll kicks off all of the passed controllers with DefaultThreadsPerController.
func StartAll(ctx context.Context, controllers ...*Impl) {
	wg := sync.WaitGroup{}
	// Start all of the controllers.
	for _, ctrlr := range controllers {
		wg.Add(1)
		concurrency := ctrlr.Concurrency
		go func(c *Impl) {
			defer wg.Done()
			c.RunContext(ctx, concurrency)
		}(ctrlr)
	}
	wg.Wait()
}

// This is attached to contexts passed to controller constructors to associate
// a resync period.
type resyncPeriodKey struct{}

// WithResyncPeriod associates the given resync period with the given context in
// the context that is returned.
func WithResyncPeriod(ctx context.Context, resync time.Duration) context.Context {
	return context.WithValue(ctx, resyncPeriodKey{}, resync)
}

// GetResyncPeriod returns the resync period associated with the given context.
// When none is specified a default resync period is used.
func GetResyncPeriod(ctx context.Context) time.Duration {
	rp := ctx.Value(resyncPeriodKey{})
	if rp == nil {
		return DefaultResyncPeriod
	}
	return rp.(time.Duration)
}

// GetTrackerLease fetches the tracker lease from the controller context.
func GetTrackerLease(ctx context.Context) time.Duration {
	return 3 * GetResyncPeriod(ctx)
}

// trackerKey is used to associate tracker.Interface with contexts.
type trackerKey struct{}

// WithTracker attaches the given tracker.Interface to the provided context
// in the returned context.
func WithTracker(ctx context.Context, t tracker.Interface) context.Context {
	return context.WithValue(ctx, trackerKey{}, t)
}

// GetTracker attempts to look up the tracker.Interface on a given context.
// It may return null if none is found.
func GetTracker(ctx context.Context) tracker.Interface {
	untyped := ctx.Value(trackerKey{})
	if untyped == nil {
		return nil
	}
	return untyped.(tracker.Interface)
}

// erKey is used to associate record.EventRecorders with contexts.
type erKey struct{}

// WithEventRecorder attaches the given record.EventRecorder to the provided context
// in the returned context.
func WithEventRecorder(ctx context.Context, er record.EventRecorder) context.Context {
	return context.WithValue(ctx, erKey{}, er)
}

// GetEventRecorder attempts to look up the record.EventRecorder on a given context.
// It may return null if none is found.
func GetEventRecorder(ctx context.Context) record.EventRecorder {
	untyped := ctx.Value(erKey{})
	if untyped == nil {
		return nil
	}
	return untyped.(record.EventRecorder)
}

func safeKey(key types.NamespacedName) string {
	if key.Namespace == "" {
		return key.Name
	}
	return key.String()
}
