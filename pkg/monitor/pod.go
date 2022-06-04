package monitor

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/informers"

	"github.com/openshift/origin/pkg/monitor/podipcontroller"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startPodMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {

	// Keep track of the condition where "pod has been pending longer than a minute"
	podInformer := cache.NewSharedIndexInformer(
		NewErrorRecordingListWatcher(m, &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.CoreV1().Pods("").List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.CoreV1().Pods("").Watch(ctx, options)
			},
		}),
		&corev1.Pod{},
		time.Hour,
		nil,
	)
	m.AddSampler(func(now time.Time) []*monitorapi.Condition {
		var conditions []*monitorapi.Condition
		for _, obj := range podInformer.GetStore().List() {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				continue
			}
			if pod.Status.Phase == "Pending" {
				if now.Sub(pod.CreationTimestamp.Time) > time.Minute {
					conditions = append(conditions, &monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: monitorapi.LocatePod(pod),
						Message: "pod has been pending longer than a minute",
					})
				}
			}
		}
		return conditions
	})
	go podInformer.Run(ctx.Done())

	podScheduledFn := func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
		oldPodHasNode := oldPod != nil && len(oldPod.Spec.NodeName) > 0
		newPodHasNode := pod != nil && len(pod.Spec.NodeName) > 0
		if !oldPodHasNode && newPodHasNode {
			return []monitorapi.Condition{
				{
					Level:   monitorapi.Info,
					Locator: monitorapi.LocatePod(pod),
					Message: monitorapi.ReasonedMessage(monitorapi.PodReasonScheduled, fmt.Sprintf("node/%s", pod.Spec.NodeName)),
				},
			}
		}
		return nil
	}

	containerStatusesReadinessFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Condition {
		isCreate := oldContainerStatuses == nil

		conditions := []monitorapi.Condition{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			newContainerReady := containerStatus.Ready
			oldContainerReady := false
			if oldContainerStatuses != nil {
				if oldContainerStatus := findContainerStatus(oldContainerStatuses, containerName, i); oldContainerStatus != nil {
					oldContainerReady = oldContainerStatus.Ready
				}
			}

			// always produce conditions during create
			if (isCreate && !newContainerReady) || (oldContainerReady && !newContainerReady) {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: monitorapi.LocatePodContainer(pod, containerName),
					Message: monitorapi.ReasonedMessage(monitorapi.ContainerReasonNotReady),
				})
			}
			if (isCreate && newContainerReady) || (!oldContainerReady && newContainerReady) {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.LocatePodContainer(pod, containerName),
					Message: monitorapi.ReasonedMessage(monitorapi.ContainerReasonReady),
				})
			}
		}

		return conditions
	}

	containerReadinessFn := func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
		isCreate := oldPod == nil

		conditions := []monitorapi.Condition{}
		if isCreate {
			conditions = append(conditions, containerStatusesReadinessFn(pod, pod.Status.ContainerStatuses, nil)...)
			conditions = append(conditions, containerStatusesReadinessFn(pod, pod.Status.InitContainerStatuses, nil)...)
		} else {
			conditions = append(conditions, containerStatusesReadinessFn(pod, pod.Status.ContainerStatuses, oldPod.Status.ContainerStatuses)...)
			conditions = append(conditions, containerStatusesReadinessFn(pod, pod.Status.InitContainerStatuses, oldPod.Status.InitContainerStatuses)...)
		}

		return conditions
	}

	containerStatusContainerWaitFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Condition {
		conditions := []monitorapi.Condition{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			// if this container is not waiting, then we don't need to compute an event for it.
			if containerStatus.State.Waiting == nil {
				continue
			}

			switch {
			// always produce messages if we have no previous container status
			// this happens on create and when a new container status appears as the kubelet starts working on it
			case oldContainerStatus == nil:
				conditions = append(conditions,
					conditionsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerWait, containerStatus.State.Waiting.Reason, " "+containerStatus.State.Waiting.Message)...)

			case oldContainerStatus.State.Waiting == nil || // if the container wasn't previously waiting OR
				containerStatus.State.Waiting.Reason != oldContainerStatus.State.Waiting.Reason: // the container was previously waiting for a different reason
				conditions = append(conditions,
					conditionsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerWait, containerStatus.State.Waiting.Reason, " "+containerStatus.State.Waiting.Message)...)
			}
		}

		return conditions
	}

	containerStatusContainerStartFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Condition {
		conditions := []monitorapi.Condition{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			// if this container is not running, then we don't need to compute an event for it.
			if containerStatus.State.Running == nil {
				continue
			}

			switch {
			// always produce messages if we have no previous container status
			// this happens on create and when a new container status appears as the kubelet starts working on it
			case oldContainerStatus == nil:
				conditions = append(conditions,
					conditionsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerStart, "", "")...)

			case oldContainerStatus.State.Running == nil: // the container was previously not running
				conditions = append(conditions,
					conditionsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerStart, "", "")...)
			}
		}

		return conditions
	}

	containerStatusContainerExitFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Condition {
		conditions := []monitorapi.Condition{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			if oldContainerStatus != nil && oldContainerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated == nil {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePodContainer(pod, containerName),
					Message: fmt.Sprintf("reason/TerminationStateCleared lastState.terminated was cleared on a pod (bug https://bugzilla.redhat.com/show_bug.cgi?id=1933760 or similar)"),
				})
			}

			// if this container is not terminated, then we don't need to compute an event for it.
			lastTerminated := containerStatus.LastTerminationState.Terminated != nil
			currentTerminated := containerStatus.State.Terminated != nil
			if !lastTerminated && !currentTerminated {
				continue
			}

			switch {
			case oldContainerStatus == nil:
				// if we have no container status, then this is probably an initial list and we missed the initial start.  Don't emit the
				// the container exit because it will be a disconnected event that can confuse our container lifecycle ordering based
				// on the event stream

			case lastTerminated && oldContainerStatus.LastTerminationState.Terminated == nil:
				// if we are transitioning to a terminated state
				if containerStatus.LastTerminationState.Terminated.ExitCode != 0 {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: monitorapi.LocatePodContainer(pod, containerName),
						Message: monitorapi.ReasonedMessagef(monitorapi.ContainerReasonContainerExit, "code/%d cause/%s %s", containerStatus.LastTerminationState.Terminated.ExitCode, containerStatus.LastTerminationState.Terminated.Reason, containerStatus.LastTerminationState.Terminated.Message),
					})
				} else {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: monitorapi.LocatePodContainer(pod, containerName),
						Message: monitorapi.ReasonedMessagef(monitorapi.ContainerReasonContainerExit, "code/0 cause/%s %s", containerStatus.LastTerminationState.Terminated.Reason, containerStatus.LastTerminationState.Terminated.Message),
					})
				}

			case currentTerminated && oldContainerStatus.State.Terminated == nil:
				// if we are transitioning to a terminated state
				if containerStatus.State.Terminated.ExitCode != 0 {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: monitorapi.LocatePodContainer(pod, containerName),
						Message: monitorapi.ReasonedMessagef(monitorapi.ContainerReasonContainerExit, "code/%d cause/%s %s", containerStatus.State.Terminated.ExitCode, containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message),
					})
				} else {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: monitorapi.LocatePodContainer(pod, containerName),
						Message: monitorapi.ReasonedMessagef(monitorapi.ContainerReasonContainerExit, "code/0 cause/%s %s", containerStatus.State.Terminated.Reason, containerStatus.State.Terminated.Message),
					})
				}
			}

		}

		return conditions
	}

	containerStatusContainerRestartedFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Condition {
		conditions := []monitorapi.Condition{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			// if this container has not been restarted, do not produce an event for it
			if containerStatus.RestartCount == 0 {
				continue
			}

			// if we have nothing to come the restart count against, do not produce an event for it.
			if oldContainerStatus == nil {
				continue
			}

			if containerStatus.RestartCount != oldContainerStatus.RestartCount {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: monitorapi.LocatePodContainer(pod, containerName),
					Message: "reason/Restarted",
				})
			}
		}

		return conditions
	}

	containerLifecycleStateFn := func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
		var oldContainerStatus []corev1.ContainerStatus
		var oldInitContainerStatus []corev1.ContainerStatus
		if oldPod != nil {
			oldContainerStatus = oldPod.Status.ContainerStatuses
			oldInitContainerStatus = oldPod.Status.InitContainerStatuses
		}

		conditions := []monitorapi.Condition{}
		conditions = append(conditions, containerStatusContainerWaitFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)
		conditions = append(conditions, containerStatusContainerStartFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)
		conditions = append(conditions, containerStatusContainerExitFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)
		conditions = append(conditions, containerStatusContainerRestartedFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)

		conditions = append(conditions, containerStatusContainerWaitFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)
		conditions = append(conditions, containerStatusContainerStartFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)
		conditions = append(conditions, containerStatusContainerExitFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)
		conditions = append(conditions, containerStatusContainerRestartedFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)

		return conditions
	}

	podCreatedFns := []func(pod *corev1.Pod) []monitorapi.Condition{
		func(pod *corev1.Pod) []monitorapi.Condition {
			return []monitorapi.Condition{
				{
					Level:   monitorapi.Info,
					Locator: monitorapi.LocatePod(pod),
					Message: monitorapi.ReasonedMessage(monitorapi.PodReasonCreated),
				},
			}
		},
		func(pod *corev1.Pod) []monitorapi.Condition {
			return podScheduledFn(pod, nil)
		},
		func(pod *corev1.Pod) []monitorapi.Condition {
			return containerLifecycleStateFn(pod, nil)
		},
		func(pod *corev1.Pod) []monitorapi.Condition {
			return containerReadinessFn(pod, nil)
		},
	}

	podChangeFns := []func(pod, oldPod *corev1.Pod) []monitorapi.Condition{
		// check if the pod was scheduled
		podScheduledFn,
		// check if container lifecycle state changed
		containerLifecycleStateFn,
		// check if readiness for containers changed
		containerReadinessFn,
		// check phase transitions
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			new, old := pod.Status.Phase, oldPod.Status.Phase
			if new == old || len(old) == 0 {
				return nil
			}
			var conditions []monitorapi.Condition
			switch {
			case new == corev1.PodPending && old != corev1.PodUnknown:
				switch {
				case pod.DeletionTimestamp != nil:
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: monitorapi.LocatePod(pod),
						Message: fmt.Sprintf("invariant violation (bug): pod should not transition %s->%s even when terminated", old, new),
					})
				case isMirrorPod(pod):
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: monitorapi.LocatePod(pod),
						Message: fmt.Sprintf("invariant violation (bug): static pod should not transition %s->%s with same UID", old, new),
					})
				default:
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: monitorapi.LocatePod(pod),
						Message: fmt.Sprintf("pod moved back to Pending"),
					})
				}
			case new == corev1.PodUnknown:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: monitorapi.LocatePod(pod),
					Message: fmt.Sprintf("pod moved to the Unknown phase"),
				})
			case new == corev1.PodFailed && old != corev1.PodFailed:
				switch pod.Status.Reason {
				case "Evicted":
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: monitorapi.LocatePod(pod),
						Message: fmt.Sprintf("reason/Evicted %s", pod.Status.Message),
					})
				case "Preempting":
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: monitorapi.LocatePod(pod),
						Message: fmt.Sprintf("reason/Preempted %s", pod.Status.Message),
					})
				default:
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: monitorapi.LocatePod(pod),
						Message: fmt.Sprintf("reason/Failed (%s): %s", pod.Status.Reason, pod.Status.Message),
					})
				}
				for _, s := range pod.Status.InitContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Error,
							Locator: monitorapi.LocatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("init container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
						})
					}
				}
				for _, s := range pod.Status.ContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Error,
							Locator: monitorapi.LocatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
						})
					}
				}
			}
			return conditions
		},
		// check for transitions to being deleted
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			var conditions []monitorapi.Condition
			if pod.DeletionGracePeriodSeconds != nil && oldPod.DeletionGracePeriodSeconds == nil {
				switch {
				case len(pod.Spec.NodeName) == 0:
					// pods that have not been assigned to a node are deleted immediately
				case pod.Status.Phase == corev1.PodFailed, pod.Status.Phase == corev1.PodSucceeded:
					// terminal pods are immediately deleted (do not undergo graceful deletion)
				default:
					if *pod.DeletionGracePeriodSeconds == 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Info,
							Locator: monitorapi.LocatePod(pod),
							Message: fmt.Sprintf("reason/ForceDelete mirrored/%t", isMirrorPod(pod)),
						})
					} else {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Info,
							Locator: monitorapi.LocatePod(pod),
							Message: monitorapi.ReasonedMessagef(monitorapi.PodReasonGracefulDeleteStarted, "duration/%ds", *pod.DeletionGracePeriodSeconds),
						})
					}
				}
			}
			if pod.DeletionGracePeriodSeconds == nil && oldPod.DeletionGracePeriodSeconds != nil {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(pod),
					Message: "invariant violation: pod was marked for deletion and then deletion grace period was cleared",
				})
			}
			return conditions
		},
		// check restarts, readiness drop outs, or other status changes
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			var conditions []monitorapi.Condition

			// container status should never be removed since the kubelet should be
			// synthesizing status from the apiserver in order to determine what to
			// run after a reboot (this is likely to be the result of a pod going back
			// to pending status)
			if len(pod.Status.ContainerStatuses) < len(oldPod.Status.ContainerStatuses) {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(pod),
					Message: "invariant violation: container statuses were removed",
				})
			}
			if len(pod.Status.InitContainerStatuses) < len(oldPod.Status.InitContainerStatuses) {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(pod),
					Message: "invariant violation: init container statuses were removed",
				})
			}

			return conditions
		},
		// inform when a pod gets reassigned to a new node
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			var conditions []monitorapi.Condition
			if len(oldPod.Spec.NodeName) > 0 && pod.Spec.NodeName != oldPod.Spec.NodeName {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: monitorapi.LocatePod(pod),
					Message: fmt.Sprintf("invariant violation, pod once assigned to a node must stay on it. The pod previously scheduled to %s, has just been assigned to a new node %s", oldPod.Spec.NodeName, pod.Spec.NodeName),
				})
			}
			return conditions
		},
	}
	podDeleteFns := []func(pod *corev1.Pod) []monitorapi.Condition{
		// check for transitions to being deleted
		func(pod *corev1.Pod) []monitorapi.Condition {
			conditions := []monitorapi.Condition{
				{
					Level:   monitorapi.Info,
					Locator: monitorapi.LocatePod(pod),
					Message: monitorapi.ReasonedMessage(monitorapi.PodReasonDeleted),
				},
			}
			switch {
			case len(pod.Spec.NodeName) == 0:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.LocatePod(pod),
					Message: monitorapi.ReasonedMessage(monitorapi.PodReasonDeletedBeforeScheduling),
				})
			case pod.Status.Phase == corev1.PodFailed, pod.Status.Phase == corev1.PodSucceeded:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: monitorapi.LocatePod(pod),
					Message: monitorapi.ReasonedMessage(monitorapi.PodReasonDeletedAfterCompletion),
				})
			default:
			}
			return conditions
		},
	}

	listWatch := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "pods", "", fields.Everything())
	customStore := newMonitoringStore(
		"pods",
		toCreateFns(podCreatedFns),
		toUpdateFns(podChangeFns),
		toDeleteFns(podDeleteFns),
		m,
		m,
	)
	reflector := cache.NewReflector(listWatch, &corev1.Pod{}, customStore, 0)
	go reflector.Run(ctx.Done())

	// start controller to watch for shared pod IPs.
	sharedInformers := informers.NewSharedInformerFactory(client, 24*time.Hour)
	podIPController := podipcontroller.NewSimultaneousPodIPController(m, sharedInformers.Core().V1().Pods())
	go podIPController.Run(ctx)
	go sharedInformers.Start(ctx.Done())

}

// toCreateFns takes a slice of functions and returns a slice of objCreateFuncs
// type objCreateFunc func(obj interface{}) []monitorapi.Condition
func toCreateFns(podCreateFns []func(pod *corev1.Pod) []monitorapi.Condition) []objCreateFunc {
	ret := []objCreateFunc{}

	for i := range podCreateFns {
		fn := podCreateFns[i]

		// We are returning a function with a signature that is accepted by something else
		// (a particular interface we are trying to satisfy perhaps?) by returning the same
		// function with the parameter casted as the one we are converting.  I'm betting
		// we wrote the origin function with corev1.Pod to take advantage of strong type
		// checking.
		ret = append(ret, func(obj interface{}) []monitorapi.Condition {
			return fn(obj.(*corev1.Pod))
		})
	}

	return ret
}

// toDeleteFns takes a slice of functions and returns a slice of objDeleteFuncs
// type objDeleteFunc func(obj interface{}) []monitorapi.Condition
func toDeleteFns(podDeleteFns []func(pod *corev1.Pod) []monitorapi.Condition) []objDeleteFunc {
	ret := []objDeleteFunc{}

	for i := range podDeleteFns {
		fn := podDeleteFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Condition {
			return fn(obj.(*corev1.Pod))
		})
	}

	return ret
}

// toUpdateFns takes a slace of functions and returns a slice of objUpdateFuncs
// type objUpdateFunc func(obj, oldObj interface{}) []monitorapi.Condition
func toUpdateFns(podUpdateFns []func(pod, oldPod *corev1.Pod) []monitorapi.Condition) []objUpdateFunc {
	ret := []objUpdateFunc{}

	for i := range podUpdateFns {
		fn := podUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Condition {
			if oldObj == nil {
				return fn(obj.(*corev1.Pod), nil)
			}
			return fn(obj.(*corev1.Pod), oldObj.(*corev1.Pod))
		})
	}

	return ret
}

func lastContainerTimeFromStatus(current *corev1.ContainerStatus) time.Time {
	if state := current.LastTerminationState.Terminated; state != nil && !state.FinishedAt.Time.IsZero() {
		return state.FinishedAt.Time
	}
	if state := current.State.Terminated; state != nil && !state.FinishedAt.Time.IsZero() {
		return state.FinishedAt.Time
	}
	return time.Time{}
}

func conditionsForTransitioningContainer(pod *corev1.Pod, current *corev1.ContainerStatus, reason, cause, message string) []monitorapi.Condition {
	return []monitorapi.Condition{
		{
			Level:   monitorapi.Info,
			Locator: monitorapi.LocatePodContainer(pod, current.Name),
			Message: monitorapi.ReasonedMessagef(reason, "cause/%s: %s", cause, message),
		},
	}
}

func isMirrorPod(pod *corev1.Pod) bool {
	return len(pod.Annotations["kubernetes.io/config.mirror"]) > 0
}
