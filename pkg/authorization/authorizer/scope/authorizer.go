package scope

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	defaultauthorizer "github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type scopeAuthorizer struct {
	delegate            defaultauthorizer.Authorizer
	clusterPolicyGetter rulevalidation.ClusterPolicyGetter
}

func NewAuthorizer(delegate defaultauthorizer.Authorizer, clusterPolicyGetter rulevalidation.ClusterPolicyGetter) defaultauthorizer.Authorizer {
	return &scopeAuthorizer{delegate: delegate, clusterPolicyGetter: clusterPolicyGetter}
}

func (a *scopeAuthorizer) Authorize(ctx kapi.Context, passedAttributes defaultauthorizer.AuthorizationAttributes) (bool, string, error) {
	user, exists := kapi.UserFrom(ctx)
	if !exists {
		return false, "", fmt.Errorf("user missing from context")
	}

	scopes := user.GetExtra()[authorizationapi.ScopesKey]
	if len(scopes) == 0 {
		return a.delegate.Authorize(ctx, passedAttributes)
	}

	nonFatalErrors := []error{}

	namespace, _ := kapi.NamespaceFrom(ctx)
	// scopeResolutionErrors aren't fatal.  If any of the scopes we find allow this, then the overall scope limits allow it
	rules, err := ScopesToRules(scopes, namespace, a.clusterPolicyGetter)
	if err != nil {
		nonFatalErrors = append(nonFatalErrors, err)
	}

	attributes := defaultauthorizer.CoerceToDefaultAuthorizationAttributes(passedAttributes)

	for _, rule := range rules {
		// check rule against attributes
		matches, err := attributes.RuleMatches(rule)
		if err != nil {
			nonFatalErrors = append(nonFatalErrors, err)
			continue
		}
		if matches {
			return a.delegate.Authorize(ctx, passedAttributes)
		}
	}

	return false, fmt.Sprintf("scopes %v, do not allow this action", scopes), kerrors.NewAggregate(nonFatalErrors)
}

// TODO remove this. We don't logically need it, but it requires splitting our interface
// GetAllowedSubjects returns the subjects it knows can perform the action.
func (a *scopeAuthorizer) GetAllowedSubjects(ctx kapi.Context, attributes defaultauthorizer.AuthorizationAttributes) (sets.String, sets.String, error) {
	return a.delegate.GetAllowedSubjects(ctx, attributes)
}
