package util

import (
	"context"
	"fmt"
	"sync"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type OperatorProgressingStatus struct {
	lock sync.Mutex

	// rolloutStableAtBeginning is closed once the operator is confirmed to be stable before progressing
	rolloutStableAtBeginning chan struct{}

	// rolloutStarted is closed once the operator starts progressing
	rolloutStarted chan struct{}

	// rolloutDone is closed once the operator finishes progressing *or* once the operation has failed.
	// If the operation failed, then RolloutError will be non-nil
	rolloutDone chan struct{}

	setErrCalled bool
	rolloutError error
}

// StableBeforeStarting is closed once the operator indicates that it is stable to begin
// returing a channel that is to be read from
func (p *OperatorProgressingStatus) StableBeforeStarting() <-chan struct{} {
	return p.rolloutStableAtBeginning
}

// Started is closed once the operator starts progressing
func (p *OperatorProgressingStatus) Started() <-chan struct{} {
	return p.rolloutStarted
}

// Done is closed once the operator finishes progressing *or* once the operation has failed.
// If the operation failed, then Err() will be non-nil
// I would've called this rolloutDone instead of Done to avoid people thinking it's a ctx.Done()
// idiom.
func (p *OperatorProgressingStatus) Done() <-chan struct{} {
	return p.rolloutDone
}

// Err returns whether or not there was failure waiting on the operator status.
// If Done is not yet closed, Err returns nil.
// If Done is closed, Err returns nil if it was successful or non-nil if it was not.
func (p *OperatorProgressingStatus) Err() error {
	select {
	case <-p.Done():
	default:
		return nil
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	err := p.rolloutError
	return err
}

func (p *OperatorProgressingStatus) setErr(err error) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.setErrCalled {
		return fmt.Errorf("setErr already called")
	}

	select {
	case <-p.Done():
		return fmt.Errorf("setErr called AFTER already done")
	default:
	}

	p.rolloutError = err
	return nil
}

func WaitForOperatorProgressingFalse(ctx context.Context, configClient configv1client.Interface, operatorName string) error {
	return waitForOperatorProgressingToBe(ctx, configClient, operatorName, false)
}

func WaitForOperatorProgressingTrue(ctx context.Context, configClient configv1client.Interface, operatorName string) error {
	return waitForOperatorProgressingToBe(ctx, configClient, operatorName, true)
}

// waitForOperatorProgressingToBe waits for the clusteroperator called operatorName to be either in status.condition
// of progressing or not progressing.  It establishes a listWatch for all clusteroperators (not sure why we didn't
// restrict it to namespace) and blocks until it receives an event that, when passed to the anonymous function, returns
// true.
//
// For any event that happens on any clusterOperator, the anonymous function is called with that event, it only cares
// about add or modify events; that func returns true when it's appropriate for the NewListWatchFromClient to exit.
func waitForOperatorProgressingToBe(ctx context.Context, configClient configv1client.Interface, operatorName string, desiredProgressing bool) error {
	_, err := watchtools.UntilWithSync(ctx,
		cache.NewListWatchFromClient(configClient.ConfigV1().RESTClient(), "clusteroperators", "", fields.Everything()),
		&configv1.ClusterOperator{},
		nil,
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Added, watch.Modified:
				operator := event.Object.(*configv1.ClusterOperator)
				if operator.Name != operatorName {
					return false, nil
				}

				// If we are waiting for Operator to be Progressing
				if desiredProgressing {
					if v1helpers.IsStatusConditionTrue(operator.Status.Conditions, configv1.OperatorProgressing) {
						return true, nil
					}
					return false, nil
				}

				// If we are waiting for Operator to not be Progressing
				if v1helpers.IsStatusConditionFalse(operator.Status.Conditions, configv1.OperatorProgressing) {
					return true, nil
				}
				return false, nil

			default:
				return false, nil
			}
		},
	)

	return err
}

// WaitForOperatorToRollout is called *before* a configuration change is made.  This method will close the first returned channel
// when the operator starts progressing and second channel once it is done progressing.  If it fails, it will panic.
//
func WaitForOperatorToRollout(ctx context.Context, configClient configv1client.Interface, operatorName string) *OperatorProgressingStatus {
	ret := &OperatorProgressingStatus{
		rolloutStableAtBeginning: make(chan struct{}),
		rolloutStarted:           make(chan struct{}),
		rolloutDone:              make(chan struct{}),
	}
	go func() {
		var err error

		// Callers wait on the ret.rolloutDone channel; the caller will unblock when ret.rolloutDone
		// is closed (i.e., when the clusterOperator is done progressing).
		defer close(ret.rolloutDone)

		// At the end of this go routing, if there's an error, we panic.
		defer func() {
			if err := ret.setErr(err); err != nil {
				panic(err)
			}
		}()

		// Create a Watch on this clusterOperator and wait until the Watch is finishes (i.e., block
		// here, until the clusterOperator is up).  When it's up, we close the channel so that
		// the thing waiting for the clusterOperator to be up can unblock and make its changes.
		err = WaitForOperatorProgressingFalse(ctx, configClient, operatorName)
		close(ret.rolloutStableAtBeginning)
		if err != nil {
			// rolloutDone and rolloutErr are set on return by defer
			return
		}

		// During this time, the caller is unblocked and doing something that may cause this
		// clusterOperator to change and become unstable while the changes are being reconciled.

		// Create a Watch on this clusterOperator and wait until the Watch finishes (i.e., block
		// here until the clusterOperator is making progress).
		err = WaitForOperatorProgressingTrue(ctx, configClient, operatorName)
		close(ret.rolloutStarted)
		if err != nil {
			// rolloutDone and rolloutErr are set on return by defer
			return
		}

		// During this time, the caller is  waiting until this clusterOperator has finished progressing
		// (i.e., the caller made changes that need to be reconciled and so the clusterOperator starts
		// progressing; when it's finished and stable, as in no longer changing, we finish) and the
		// defer close above runs to close this channel -- unblocking the caller who waited on it.
		err = WaitForOperatorProgressingFalse(ctx, configClient, operatorName)
		// rolloutDone and rolloutErr are set on return by defer
	}()

	return ret
}
