// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/suse/carrier/shim/pkg/gitea"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/suse/carrier/shim/models"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/app_usage_events"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/apps"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/auth"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/buildpacks"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/domains_deprecated"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/environment_variable_groups"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/events"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/feature_flags"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/files"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/info"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/jobs"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/organization_quota_definitions"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/organizations"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/private_domains"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/resource_match"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/routes"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/security_group_running_defaults"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/security_group_staging_defaults"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/security_groups"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_auth_tokens_deprecated"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_bindings"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_brokers"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_instances"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_plan_visibilities"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_plans"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/service_usage_events_experimental"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/services"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/shared_domains"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/space_quota_definitions"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/spaces"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/stacks"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/user_provided_service_instances"
	"github.com/suse/carrier/shim/restapi/carrier_shim_cf/users"
)

//go:generate swagger generate server --target ../../shim --name CloudFoundry --spec ../cc-swagger-v2.yaml --api-package carrier-shim-cf --principal interface{} --exclude-main

func configureFlags(api *carrier_shim_cf.CloudFoundryAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *carrier_shim_cf.CloudFoundryAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.UseSwaggerUI()
	// To continue using redoc as your UI, uncomment the following line
	// api.UseRedoc()

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	api.UrlformConsumer = runtime.DiscardConsumer

	// Additions for the authentication API
	api.AuthGetAuthLoginHandler = auth.GetAuthLoginHandlerFunc(func(params auth.GetAuthLoginParams) middleware.Responder {
		return auth.NewGetAuthLoginOK().WithPayload(
			&models.RetrieveAuthLogin{
				App: &models.RetrieveAuthLoginApp{
					Version: "0.0.0",
				},
				Links: &models.RetrieveAuthLoginLinks{
					Uaa:      "http://127.0.0.1:9292/",
					Login:    "http://127.0.0.1:9292/",
					Register: "/create_account",
					Passwd:   "/forgot_password",
				},
				ZoneName:       "carrier",
				EntityID:       "carrier",
				CommitID:       "deadbeef",
				IdpDefinitions: map[string]string{},
				Timestamp:      "2020-08-18T20:57:15+0000",
				Prompts: &models.RetrieveAuthLoginPrompts{
					Username: []interface{}{"text", "Email"},
					Password: []interface{}{"password", "Password"},
					Passcode: []interface{}{},
				},
			})
	})
	api.AuthPostAuthOauthTokenHandler = auth.PostAuthOauthTokenHandlerFunc(func(params auth.PostAuthOauthTokenParams) middleware.Responder {
		return auth.NewPostOauthTokenOK().WithPayload(
			&models.CreatesOAuthTokenResponse{
				AccessToken:  "foo",
				ExpiresIn:    36000,
				IDToken:      "foo_id",
				Jti:          "f183ad52-71a1-4591-947d-024b387dea70",
				RefreshToken: "bar",
				Scope:        "read write",
				TokenType:    "bearer",
			})
	})

	// Cloud Foundry v2 API
	if api.RoutesAssociateAppWithRouteHandler == nil {
		api.RoutesAssociateAppWithRouteHandler = routes.AssociateAppWithRouteHandlerFunc(func(params routes.AssociateAppWithRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.AssociateAppWithRoute has not yet been implemented")
		})
	}
	if api.UsersAssociateAuditedOrganizationWithUserHandler == nil {
		api.UsersAssociateAuditedOrganizationWithUserHandler = users.AssociateAuditedOrganizationWithUserHandlerFunc(func(params users.AssociateAuditedOrganizationWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateAuditedOrganizationWithUser has not yet been implemented")
		})
	}
	if api.UsersAssociateAuditedSpaceWithUserHandler == nil {
		api.UsersAssociateAuditedSpaceWithUserHandler = users.AssociateAuditedSpaceWithUserHandlerFunc(func(params users.AssociateAuditedSpaceWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateAuditedSpaceWithUser has not yet been implemented")
		})
	}
	if api.OrganizationsAssociateAuditorWithOrganizationHandler == nil {
		api.OrganizationsAssociateAuditorWithOrganizationHandler = organizations.AssociateAuditorWithOrganizationHandlerFunc(func(params organizations.AssociateAuditorWithOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.AssociateAuditorWithOrganization has not yet been implemented")
		})
	}
	if api.SpacesAssociateAuditorWithSpaceHandler == nil {
		api.SpacesAssociateAuditorWithSpaceHandler = spaces.AssociateAuditorWithSpaceHandlerFunc(func(params spaces.AssociateAuditorWithSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.AssociateAuditorWithSpace has not yet been implemented")
		})
	}
	if api.UsersAssociateBillingManagedOrganizationWithUserHandler == nil {
		api.UsersAssociateBillingManagedOrganizationWithUserHandler = users.AssociateBillingManagedOrganizationWithUserHandlerFunc(func(params users.AssociateBillingManagedOrganizationWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateBillingManagedOrganizationWithUser has not yet been implemented")
		})
	}
	if api.OrganizationsAssociateBillingManagerWithOrganizationHandler == nil {
		api.OrganizationsAssociateBillingManagerWithOrganizationHandler = organizations.AssociateBillingManagerWithOrganizationHandlerFunc(func(params organizations.AssociateBillingManagerWithOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.AssociateBillingManagerWithOrganization has not yet been implemented")
		})
	}

	if api.SpacesAssociateDeveloperWithSpaceHandler == nil {
		api.SpacesAssociateDeveloperWithSpaceHandler =
			spaces.AssociateDeveloperWithSpaceHandlerFunc(func(params spaces.AssociateDeveloperWithSpaceParams) middleware.Responder {
				return middleware.NotImplemented("operation spaces.AssociateDeveloperWithSpace has not yet been implemented")
			})
	}

	if api.UsersAssociateManagedOrganizationWithUserHandler == nil {
		api.UsersAssociateManagedOrganizationWithUserHandler = users.AssociateManagedOrganizationWithUserHandlerFunc(func(params users.AssociateManagedOrganizationWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateManagedOrganizationWithUser has not yet been implemented")
		})
	}
	if api.UsersAssociateManagedSpaceWithUserHandler == nil {
		api.UsersAssociateManagedSpaceWithUserHandler = users.AssociateManagedSpaceWithUserHandlerFunc(func(params users.AssociateManagedSpaceWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateManagedSpaceWithUser has not yet been implemented")
		})
	}
	if api.OrganizationsAssociateManagerWithOrganizationHandler == nil {
		api.OrganizationsAssociateManagerWithOrganizationHandler = organizations.AssociateManagerWithOrganizationHandlerFunc(func(params organizations.AssociateManagerWithOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.AssociateManagerWithOrganization has not yet been implemented")
		})
	}
	if api.SpacesAssociateManagerWithSpaceHandler == nil {
		api.SpacesAssociateManagerWithSpaceHandler = spaces.AssociateManagerWithSpaceHandlerFunc(func(params spaces.AssociateManagerWithSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.AssociateManagerWithSpace has not yet been implemented")
		})
	}
	if api.UsersAssociateOrganizationWithUserHandler == nil {
		api.UsersAssociateOrganizationWithUserHandler = users.AssociateOrganizationWithUserHandlerFunc(func(params users.AssociateOrganizationWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateOrganizationWithUser has not yet been implemented")
		})
	}

	api.AppsAssociateRouteWithAppHandler = apps.AssociateRouteWithAppHandlerFunc(func(params apps.AssociateRouteWithAppParams) middleware.Responder {
		return apps.NewAssociateRouteWithAppCreated().WithPayload(
			&models.AssociateRouteWithAppResponseResource{
				Metadata: &models.EntityMetadata{
					GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
				},
				Entity: &models.AssociateRouteWithAppResponse{
					Name: "myapp",
				},
			},
		)
	})

	if api.SpacesAssociateSecurityGroupWithSpaceHandler == nil {
		api.SpacesAssociateSecurityGroupWithSpaceHandler = spaces.AssociateSecurityGroupWithSpaceHandlerFunc(func(params spaces.AssociateSecurityGroupWithSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.AssociateSecurityGroupWithSpace has not yet been implemented")
		})
	}
	if api.SecurityGroupsAssociateSpaceWithSecurityGroupHandler == nil {
		api.SecurityGroupsAssociateSpaceWithSecurityGroupHandler = security_groups.AssociateSpaceWithSecurityGroupHandlerFunc(func(params security_groups.AssociateSpaceWithSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.AssociateSpaceWithSecurityGroup has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsAssociateSpaceWithSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsAssociateSpaceWithSpaceQuotaDefinitionHandler = space_quota_definitions.AssociateSpaceWithSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.AssociateSpaceWithSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.AssociateSpaceWithSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.UsersAssociateSpaceWithUserHandler == nil {
		api.UsersAssociateSpaceWithUserHandler = users.AssociateSpaceWithUserHandlerFunc(func(params users.AssociateSpaceWithUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.AssociateSpaceWithUser has not yet been implemented")
		})
	}
	if api.OrganizationsAssociateUserWithOrganizationHandler == nil {
		api.OrganizationsAssociateUserWithOrganizationHandler = organizations.AssociateUserWithOrganizationHandlerFunc(func(params organizations.AssociateUserWithOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.AssociateUserWithOrganization has not yet been implemented")
		})
	}
	if api.RoutesCheckRouteExistsHandler == nil {
		api.RoutesCheckRouteExistsHandler = routes.CheckRouteExistsHandlerFunc(func(params routes.CheckRouteExistsParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.CheckRouteExists has not yet been implemented")
		})
	}
	if api.AppsCopyAppBitsForAppHandler == nil {
		api.AppsCopyAppBitsForAppHandler = apps.CopyAppBitsForAppHandlerFunc(func(params apps.CopyAppBitsForAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.CopyAppBitsForApp has not yet been implemented")
		})
	}
	if api.AppsCreateAppHandler == nil {
		api.AppsCreateAppHandler = apps.CreateAppHandlerFunc(func(params apps.CreateAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.CreateApp has not yet been implemented")
		})
	}
	if api.OrganizationsCreateOrganizationHandler == nil {
		api.OrganizationsCreateOrganizationHandler = organizations.CreateOrganizationHandlerFunc(func(params organizations.CreateOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.CreateOrganization has not yet been implemented")
		})
	}
	if api.OrganizationQuotaDefinitionsCreateOrganizationQuotaDefinitionHandler == nil {
		api.OrganizationQuotaDefinitionsCreateOrganizationQuotaDefinitionHandler = organization_quota_definitions.CreateOrganizationQuotaDefinitionHandlerFunc(func(params organization_quota_definitions.CreateOrganizationQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation organization_quota_definitions.CreateOrganizationQuotaDefinition has not yet been implemented")
		})
	}
	if api.PrivateDomainsCreatePrivateDomainOwnedByGivenOrganizationHandler == nil {
		api.PrivateDomainsCreatePrivateDomainOwnedByGivenOrganizationHandler = private_domains.CreatePrivateDomainOwnedByGivenOrganizationHandlerFunc(func(params private_domains.CreatePrivateDomainOwnedByGivenOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation private_domains.CreatePrivateDomainOwnedByGivenOrganization has not yet been implemented")
		})
	}
	if api.RoutesCreateRouteHandler == nil {
		api.RoutesCreateRouteHandler = routes.CreateRouteHandlerFunc(func(params routes.CreateRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.CreateRoute has not yet been implemented")
		})
	}
	if api.SecurityGroupsCreateSecurityGroupHandler == nil {
		api.SecurityGroupsCreateSecurityGroupHandler = security_groups.CreateSecurityGroupHandlerFunc(func(params security_groups.CreateSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.CreateSecurityGroup has not yet been implemented")
		})
	}
	if api.ServiceBindingsCreateServiceBindingHandler == nil {
		api.ServiceBindingsCreateServiceBindingHandler = service_bindings.CreateServiceBindingHandlerFunc(func(params service_bindings.CreateServiceBindingParams) middleware.Responder {
			return middleware.NotImplemented("operation service_bindings.CreateServiceBinding has not yet been implemented")
		})
	}
	if api.ServiceBrokersCreateServiceBrokerHandler == nil {
		api.ServiceBrokersCreateServiceBrokerHandler = service_brokers.CreateServiceBrokerHandlerFunc(func(params service_brokers.CreateServiceBrokerParams) middleware.Responder {
			return middleware.NotImplemented("operation service_brokers.CreateServiceBroker has not yet been implemented")
		})
	}
	if api.ServicesCreateServiceDeprecatedHandler == nil {
		api.ServicesCreateServiceDeprecatedHandler = services.CreateServiceDeprecatedHandlerFunc(func(params services.CreateServiceDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation services.CreateServiceDeprecated has not yet been implemented")
		})
	}
	if api.ServiceInstancesCreateServiceInstanceHandler == nil {
		api.ServiceInstancesCreateServiceInstanceHandler = service_instances.CreateServiceInstanceHandlerFunc(func(params service_instances.CreateServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.CreateServiceInstance has not yet been implemented")
		})
	}
	if api.ServicePlansCreateServicePlanDeprecatedHandler == nil {
		api.ServicePlansCreateServicePlanDeprecatedHandler = service_plans.CreateServicePlanDeprecatedHandlerFunc(func(params service_plans.CreateServicePlanDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plans.CreateServicePlanDeprecated has not yet been implemented")
		})
	}
	if api.ServicePlanVisibilitiesCreateServicePlanVisibilityHandler == nil {
		api.ServicePlanVisibilitiesCreateServicePlanVisibilityHandler = service_plan_visibilities.CreateServicePlanVisibilityHandlerFunc(func(params service_plan_visibilities.CreateServicePlanVisibilityParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plan_visibilities.CreateServicePlanVisibility has not yet been implemented")
		})
	}
	if api.SharedDomainsCreateSharedDomainHandler == nil {
		api.SharedDomainsCreateSharedDomainHandler = shared_domains.CreateSharedDomainHandlerFunc(func(params shared_domains.CreateSharedDomainParams) middleware.Responder {
			return middleware.NotImplemented("operation shared_domains.CreateSharedDomain has not yet been implemented")
		})
	}
	if api.SpacesCreateSpaceHandler == nil {
		api.SpacesCreateSpaceHandler = spaces.CreateSpaceHandlerFunc(func(params spaces.CreateSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.CreateSpace has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsCreateSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsCreateSpaceQuotaDefinitionHandler = space_quota_definitions.CreateSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.CreateSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.CreateSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.UsersCreateUserHandler == nil {
		api.UsersCreateUserHandler = users.CreateUserHandlerFunc(func(params users.CreateUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.CreateUser has not yet been implemented")
		})
	}
	if api.UserProvidedServiceInstancesCreateUserProvidedServiceInstanceHandler == nil {
		api.UserProvidedServiceInstancesCreateUserProvidedServiceInstanceHandler = user_provided_service_instances.CreateUserProvidedServiceInstanceHandlerFunc(func(params user_provided_service_instances.CreateUserProvidedServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation user_provided_service_instances.CreateUserProvidedServiceInstance has not yet been implemented")
		})
	}
	if api.BuildpacksCreatesAdminBuildpackHandler == nil {
		api.BuildpacksCreatesAdminBuildpackHandler = buildpacks.CreatesAdminBuildpackHandlerFunc(func(params buildpacks.CreatesAdminBuildpackParams) middleware.Responder {
			return middleware.NotImplemented("operation buildpacks.CreatesAdminBuildpack has not yet been implemented")
		})
	}
	if api.DomainsDeprecatedCreatesDomainOwnedByGivenOrganizationDeprecatedHandler == nil {
		api.DomainsDeprecatedCreatesDomainOwnedByGivenOrganizationDeprecatedHandler = domains_deprecated.CreatesDomainOwnedByGivenOrganizationDeprecatedHandlerFunc(func(params domains_deprecated.CreatesDomainOwnedByGivenOrganizationDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation domains_deprecated.CreatesDomainOwnedByGivenOrganizationDeprecated has not yet been implemented")
		})
	}

	api.AppsDeleteAppHandler = apps.DeleteAppHandlerFunc(func(params apps.DeleteAppParams) middleware.Responder {
		pushInfo := gitea.PushInfo{
			AppName: "myapp",
		}

		err := gitea.Delete(pushInfo)
		if err != nil {
			return middleware.Error(504, err)
		}

		return apps.NewDeleteAppNoContent()
	})

	if api.BuildpacksDeleteBuildpackHandler == nil {
		api.BuildpacksDeleteBuildpackHandler = buildpacks.DeleteBuildpackHandlerFunc(func(params buildpacks.DeleteBuildpackParams) middleware.Responder {
			return middleware.NotImplemented("operation buildpacks.DeleteBuildpack has not yet been implemented")
		})
	}
	if api.DomainsDeprecatedDeleteDomainDeprecatedHandler == nil {
		api.DomainsDeprecatedDeleteDomainDeprecatedHandler = domains_deprecated.DeleteDomainDeprecatedHandlerFunc(func(params domains_deprecated.DeleteDomainDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation domains_deprecated.DeleteDomainDeprecated has not yet been implemented")
		})
	}
	if api.OrganizationsDeleteOrganizationHandler == nil {
		api.OrganizationsDeleteOrganizationHandler = organizations.DeleteOrganizationHandlerFunc(func(params organizations.DeleteOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.DeleteOrganization has not yet been implemented")
		})
	}
	if api.OrganizationQuotaDefinitionsDeleteOrganizationQuotaDefinitionHandler == nil {
		api.OrganizationQuotaDefinitionsDeleteOrganizationQuotaDefinitionHandler = organization_quota_definitions.DeleteOrganizationQuotaDefinitionHandlerFunc(func(params organization_quota_definitions.DeleteOrganizationQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation organization_quota_definitions.DeleteOrganizationQuotaDefinition has not yet been implemented")
		})
	}
	if api.PrivateDomainsDeletePrivateDomainHandler == nil {
		api.PrivateDomainsDeletePrivateDomainHandler = private_domains.DeletePrivateDomainHandlerFunc(func(params private_domains.DeletePrivateDomainParams) middleware.Responder {
			return middleware.NotImplemented("operation private_domains.DeletePrivateDomain has not yet been implemented")
		})
	}
	if api.RoutesDeleteRouteHandler == nil {
		api.RoutesDeleteRouteHandler = routes.DeleteRouteHandlerFunc(func(params routes.DeleteRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.DeleteRoute has not yet been implemented")
		})
	}
	if api.SecurityGroupsDeleteSecurityGroupHandler == nil {
		api.SecurityGroupsDeleteSecurityGroupHandler = security_groups.DeleteSecurityGroupHandlerFunc(func(params security_groups.DeleteSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.DeleteSecurityGroup has not yet been implemented")
		})
	}
	if api.ServicesDeleteServiceHandler == nil {
		api.ServicesDeleteServiceHandler = services.DeleteServiceHandlerFunc(func(params services.DeleteServiceParams) middleware.Responder {
			return middleware.NotImplemented("operation services.DeleteService has not yet been implemented")
		})
	}
	if api.ServiceAuthTokensDeprecatedDeleteServiceAuthTokenDeprecatedHandler == nil {
		api.ServiceAuthTokensDeprecatedDeleteServiceAuthTokenDeprecatedHandler = service_auth_tokens_deprecated.DeleteServiceAuthTokenDeprecatedHandlerFunc(func(params service_auth_tokens_deprecated.DeleteServiceAuthTokenDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation service_auth_tokens_deprecated.DeleteServiceAuthTokenDeprecated has not yet been implemented")
		})
	}
	if api.ServiceBindingsDeleteServiceBindingHandler == nil {
		api.ServiceBindingsDeleteServiceBindingHandler = service_bindings.DeleteServiceBindingHandlerFunc(func(params service_bindings.DeleteServiceBindingParams) middleware.Responder {
			return middleware.NotImplemented("operation service_bindings.DeleteServiceBinding has not yet been implemented")
		})
	}
	if api.ServiceBrokersDeleteServiceBrokerHandler == nil {
		api.ServiceBrokersDeleteServiceBrokerHandler = service_brokers.DeleteServiceBrokerHandlerFunc(func(params service_brokers.DeleteServiceBrokerParams) middleware.Responder {
			return middleware.NotImplemented("operation service_brokers.DeleteServiceBroker has not yet been implemented")
		})
	}
	if api.ServiceInstancesDeleteServiceInstanceHandler == nil {
		api.ServiceInstancesDeleteServiceInstanceHandler = service_instances.DeleteServiceInstanceHandlerFunc(func(params service_instances.DeleteServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.DeleteServiceInstance has not yet been implemented")
		})
	}
	if api.ServicePlanVisibilitiesDeleteServicePlanVisibilitiesHandler == nil {
		api.ServicePlanVisibilitiesDeleteServicePlanVisibilitiesHandler = service_plan_visibilities.DeleteServicePlanVisibilitiesHandlerFunc(func(params service_plan_visibilities.DeleteServicePlanVisibilitiesParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plan_visibilities.DeleteServicePlanVisibilities has not yet been implemented")
		})
	}
	if api.ServicePlansDeleteServicePlansHandler == nil {
		api.ServicePlansDeleteServicePlansHandler = service_plans.DeleteServicePlansHandlerFunc(func(params service_plans.DeleteServicePlansParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plans.DeleteServicePlans has not yet been implemented")
		})
	}
	if api.SharedDomainsDeleteSharedDomainHandler == nil {
		api.SharedDomainsDeleteSharedDomainHandler = shared_domains.DeleteSharedDomainHandlerFunc(func(params shared_domains.DeleteSharedDomainParams) middleware.Responder {
			return middleware.NotImplemented("operation shared_domains.DeleteSharedDomain has not yet been implemented")
		})
	}
	if api.SpacesDeleteSpaceHandler == nil {
		api.SpacesDeleteSpaceHandler = spaces.DeleteSpaceHandlerFunc(func(params spaces.DeleteSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.DeleteSpace has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsDeleteSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsDeleteSpaceQuotaDefinitionHandler = space_quota_definitions.DeleteSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.DeleteSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.DeleteSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.StacksDeleteStackHandler == nil {
		api.StacksDeleteStackHandler = stacks.DeleteStackHandlerFunc(func(params stacks.DeleteStackParams) middleware.Responder {
			return middleware.NotImplemented("operation stacks.DeleteStack has not yet been implemented")
		})
	}
	if api.UsersDeleteUserHandler == nil {
		api.UsersDeleteUserHandler = users.DeleteUserHandlerFunc(func(params users.DeleteUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.DeleteUser has not yet been implemented")
		})
	}
	if api.UserProvidedServiceInstancesDeleteUserProvidedServiceInstanceHandler == nil {
		api.UserProvidedServiceInstancesDeleteUserProvidedServiceInstanceHandler = user_provided_service_instances.DeleteUserProvidedServiceInstanceHandlerFunc(func(params user_provided_service_instances.DeleteUserProvidedServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation user_provided_service_instances.DeleteUserProvidedServiceInstance has not yet been implemented")
		})
	}
	if api.AppsDownloadsBitsForAppHandler == nil {
		api.AppsDownloadsBitsForAppHandler = apps.DownloadsBitsForAppHandlerFunc(func(params apps.DownloadsBitsForAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.DownloadsBitsForApp has not yet been implemented")
		})
	}
	if api.PrivateDomainsFilterPrivateDomainsByNameHandler == nil {
		api.PrivateDomainsFilterPrivateDomainsByNameHandler = private_domains.FilterPrivateDomainsByNameHandlerFunc(func(params private_domains.FilterPrivateDomainsByNameParams) middleware.Responder {
			return middleware.NotImplemented("operation private_domains.FilterPrivateDomainsByName has not yet been implemented")
		})
	}
	if api.ServiceAuthTokensDeprecatedFilterResultSetByLabelDeprecatedHandler == nil {
		api.ServiceAuthTokensDeprecatedFilterResultSetByLabelDeprecatedHandler = service_auth_tokens_deprecated.FilterResultSetByLabelDeprecatedHandlerFunc(func(params service_auth_tokens_deprecated.FilterResultSetByLabelDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation service_auth_tokens_deprecated.FilterResultSetByLabelDeprecated has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetAllFeatureFlagsHandler == nil {
		api.FeatureFlagsGetAllFeatureFlagsHandler = feature_flags.GetAllFeatureFlagsHandlerFunc(func(params feature_flags.GetAllFeatureFlagsParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetAllFeatureFlags has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetAppBitsUploadFeatureFlagHandler == nil {
		api.FeatureFlagsGetAppBitsUploadFeatureFlagHandler = feature_flags.GetAppBitsUploadFeatureFlagHandlerFunc(func(params feature_flags.GetAppBitsUploadFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetAppBitsUploadFeatureFlag has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetAppScalingFeatureFlagHandler == nil {
		api.FeatureFlagsGetAppScalingFeatureFlagHandler = feature_flags.GetAppScalingFeatureFlagHandlerFunc(func(params feature_flags.GetAppScalingFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetAppScalingFeatureFlag has not yet been implemented")
		})
	}

	api.AppsGetAppSummaryHandler = apps.GetAppSummaryHandlerFunc(func(params apps.GetAppSummaryParams) middleware.Responder {
		return apps.NewGetAppSummaryOK().WithPayload(
			&models.GetAppSummaryResponse{
				GUID:             "4feb778f-9d26-4332-9868-e03715fae08f",
				Name:             "myapp",
				Instances:        1,
				RunningInstances: 1,
				Routes: []models.GenericObject{
					models.GenericObject{
						"guid": "1a393e14-b12c-4b66-949d-0a89d72b7a74",
						"host": "127.0.0.1",
						"domain": models.GenericObject{
							"guid": "ca0c81a7-c6c0-43e6-a9e7-2d09f0e85bc1",
							"name": "myapp.127.0.0.1.nip.io",
						},
					},
				},
			},
		)
	})

	api.AppsGetDetailedStatsForStartedAppHandler = apps.GetDetailedStatsForStartedAppHandlerFunc(func(params apps.GetDetailedStatsForStartedAppParams) middleware.Responder {
		return apps.NewGetDetailedStatsForStartedAppOK().WithPayload(
			models.GetDetailedStatsForStartedAppResponseResource{
				"0": models.GetDetailedStatsForStartedAppResponse{
					State: "RUNNING",
					Stats: models.GenericObject{
						"name": "myapp",
						"usage": map[string]interface{}{
							"disk": 66392064,
							"mem":  29880320,
							"cpu":  0.13511219703079957,
							"time": "2014-06-19 22:37:58 +0000",
						},
					},
				},
			},
		)
	})

	if api.AppsGetEnvForAppHandler == nil {
		api.AppsGetEnvForAppHandler = apps.GetEnvForAppHandlerFunc(func(params apps.GetEnvForAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.GetEnvForApp has not yet been implemented")
		})
	}

	api.InfoGetInfoHandler = info.GetInfoHandlerFunc(func(params info.GetInfoParams) middleware.Responder {
		return info.NewGetInfoOK().WithPayload(
			&models.GetInfoResponse{
				AuthorizationEndpoint: "http://127.0.0.1:9292/v2/auth",
				LoggingEndpoint:       "ws://127.0.0.1:9292/v2/logging",
				TokenEndpoint:         "http://127.0.0.1:9292/v2/token",
			})
	})

	api.AppsGetInstanceInformationForStartedAppHandler = apps.GetInstanceInformationForStartedAppHandlerFunc(func(params apps.GetInstanceInformationForStartedAppParams) middleware.Responder {
		return apps.NewGetInstanceInformationForStartedAppOK().WithPayload(
			models.GetInstanceInformationForStartedAppResponseResource{
				"0": models.GetInstanceInformationForStartedAppResponse{
					State: "RUNNING",
					Since: 1403140717.984577,
				},
			},
		)
	})

	if api.OrganizationsGetOrganizationSummaryHandler == nil {
		api.OrganizationsGetOrganizationSummaryHandler = organizations.GetOrganizationSummaryHandlerFunc(func(params organizations.GetOrganizationSummaryParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.GetOrganizationSummary has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetPrivateDomainCreationFeatureFlagHandler == nil {
		api.FeatureFlagsGetPrivateDomainCreationFeatureFlagHandler = feature_flags.GetPrivateDomainCreationFeatureFlagHandlerFunc(func(params feature_flags.GetPrivateDomainCreationFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetPrivateDomainCreationFeatureFlag has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetRouteCreationFeatureFlagHandler == nil {
		api.FeatureFlagsGetRouteCreationFeatureFlagHandler = feature_flags.GetRouteCreationFeatureFlagHandlerFunc(func(params feature_flags.GetRouteCreationFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetRouteCreationFeatureFlag has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetServiceInstanceCreationFeatureFlagHandler == nil {
		api.FeatureFlagsGetServiceInstanceCreationFeatureFlagHandler = feature_flags.GetServiceInstanceCreationFeatureFlagHandlerFunc(func(params feature_flags.GetServiceInstanceCreationFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetServiceInstanceCreationFeatureFlag has not yet been implemented")
		})
	}
	if api.SpacesGetSpaceSummaryHandler == nil {
		api.SpacesGetSpaceSummaryHandler = spaces.GetSpaceSummaryHandlerFunc(func(params spaces.GetSpaceSummaryParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.GetSpaceSummary has not yet been implemented")
		})
	}
	if api.FeatureFlagsGetUserOrgCreationFeatureFlagHandler == nil {
		api.FeatureFlagsGetUserOrgCreationFeatureFlagHandler = feature_flags.GetUserOrgCreationFeatureFlagHandlerFunc(func(params feature_flags.GetUserOrgCreationFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.GetUserOrgCreationFeatureFlag has not yet been implemented")
		})
	}
	if api.UsersGetUserSummaryHandler == nil {
		api.UsersGetUserSummaryHandler = users.GetUserSummaryHandlerFunc(func(params users.GetUserSummaryParams) middleware.Responder {
			return middleware.NotImplemented("operation users.GetUserSummary has not yet been implemented")
		})
	}
	if api.EnvironmentVariableGroupsGettingContentsOfRunningEnvironmentVariableGroupHandler == nil {
		api.EnvironmentVariableGroupsGettingContentsOfRunningEnvironmentVariableGroupHandler = environment_variable_groups.GettingContentsOfRunningEnvironmentVariableGroupHandlerFunc(func(params environment_variable_groups.GettingContentsOfRunningEnvironmentVariableGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation environment_variable_groups.GettingContentsOfRunningEnvironmentVariableGroup has not yet been implemented")
		})
	}
	if api.EnvironmentVariableGroupsGettingContentsOfStagingEnvironmentVariableGroupHandler == nil {
		api.EnvironmentVariableGroupsGettingContentsOfStagingEnvironmentVariableGroupHandler = environment_variable_groups.GettingContentsOfStagingEnvironmentVariableGroupHandlerFunc(func(params environment_variable_groups.GettingContentsOfStagingEnvironmentVariableGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation environment_variable_groups.GettingContentsOfStagingEnvironmentVariableGroup has not yet been implemented")
		})
	}
	if api.AppUsageEventsListAllAppUsageEventsHandler == nil {
		api.AppUsageEventsListAllAppUsageEventsHandler = app_usage_events.ListAllAppUsageEventsHandlerFunc(func(params app_usage_events.ListAllAppUsageEventsParams) middleware.Responder {
			return middleware.NotImplemented("operation app_usage_events.ListAllAppUsageEvents has not yet been implemented")
		})
	}
	if api.AppsListAllAppsHandler == nil {
		api.AppsListAllAppsHandler = apps.ListAllAppsHandlerFunc(func(params apps.ListAllAppsParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.ListAllApps has not yet been implemented")
		})
	}
	if api.RoutesListAllAppsForRouteHandler == nil {
		api.RoutesListAllAppsForRouteHandler = routes.ListAllAppsForRouteHandlerFunc(func(params routes.ListAllAppsForRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.ListAllAppsForRoute has not yet been implemented")
		})
	}

	api.SpacesListAllAppsForSpaceHandler = spaces.ListAllAppsForSpaceHandlerFunc(func(params spaces.ListAllAppsForSpaceParams) middleware.Responder {
		return spaces.NewListAllAppsForSpaceOK().WithPayload(
			&models.ListAllAppsForSpaceResponsePaged{
				Resources: []*models.ListAllAppsForSpaceResponseResource{
					&models.ListAllAppsForSpaceResponseResource{
						Metadata: &models.EntityMetadata{
							GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
						},
						Entity: &models.ListAllAppsForSpaceResponse{
							Name: "myapp",
						},
					},
				},
			},
		)
	})

	if api.UsersListAllAuditedOrganizationsForUserHandler == nil {
		api.UsersListAllAuditedOrganizationsForUserHandler = users.ListAllAuditedOrganizationsForUserHandlerFunc(func(params users.ListAllAuditedOrganizationsForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllAuditedOrganizationsForUser has not yet been implemented")
		})
	}
	if api.UsersListAllAuditedSpacesForUserHandler == nil {
		api.UsersListAllAuditedSpacesForUserHandler = users.ListAllAuditedSpacesForUserHandlerFunc(func(params users.ListAllAuditedSpacesForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllAuditedSpacesForUser has not yet been implemented")
		})
	}
	if api.OrganizationsListAllAuditorsForOrganizationHandler == nil {
		api.OrganizationsListAllAuditorsForOrganizationHandler = organizations.ListAllAuditorsForOrganizationHandlerFunc(func(params organizations.ListAllAuditorsForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllAuditorsForOrganization has not yet been implemented")
		})
	}
	if api.SpacesListAllAuditorsForSpaceHandler == nil {
		api.SpacesListAllAuditorsForSpaceHandler = spaces.ListAllAuditorsForSpaceHandlerFunc(func(params spaces.ListAllAuditorsForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllAuditorsForSpace has not yet been implemented")
		})
	}
	if api.UsersListAllBillingManagedOrganizationsForUserHandler == nil {
		api.UsersListAllBillingManagedOrganizationsForUserHandler = users.ListAllBillingManagedOrganizationsForUserHandlerFunc(func(params users.ListAllBillingManagedOrganizationsForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllBillingManagedOrganizationsForUser has not yet been implemented")
		})
	}
	if api.OrganizationsListAllBillingManagersForOrganizationHandler == nil {
		api.OrganizationsListAllBillingManagersForOrganizationHandler = organizations.ListAllBillingManagersForOrganizationHandlerFunc(func(params organizations.ListAllBillingManagersForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllBillingManagersForOrganization has not yet been implemented")
		})
	}
	if api.BuildpacksListAllBuildpacksHandler == nil {
		api.BuildpacksListAllBuildpacksHandler = buildpacks.ListAllBuildpacksHandlerFunc(func(params buildpacks.ListAllBuildpacksParams) middleware.Responder {
			return middleware.NotImplemented("operation buildpacks.ListAllBuildpacks has not yet been implemented")
		})
	}
	if api.SpacesListAllDevelopersForSpaceHandler == nil {
		api.SpacesListAllDevelopersForSpaceHandler = spaces.ListAllDevelopersForSpaceHandlerFunc(func(params spaces.ListAllDevelopersForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllDevelopersForSpace has not yet been implemented")
		})
	}

	api.DomainsDeprecatedListAllDomainsDeprecatedHandler = domains_deprecated.ListAllDomainsDeprecatedHandlerFunc(func(params domains_deprecated.ListAllDomainsDeprecatedParams) middleware.Responder {
		return domains_deprecated.NewListAllDomainsDeprecatedOK().WithPayload(
			&models.ListAllDomainsDeprecatedResponsePaged{
				Resources: []*models.ListAllDomainsDeprecatedResponseResource{
					&models.ListAllDomainsDeprecatedResponseResource{
						Metadata: &models.EntityMetadata{
							GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
						},
						Entity: &models.ListAllDomainsDeprecatedResponse{
							Name: "mydomain",
						},
					},
				},
			},
		)
	})

	if api.OrganizationsListAllDomainsForOrganizationDeprecatedHandler == nil {
		api.OrganizationsListAllDomainsForOrganizationDeprecatedHandler = organizations.ListAllDomainsForOrganizationDeprecatedHandlerFunc(func(params organizations.ListAllDomainsForOrganizationDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllDomainsForOrganizationDeprecated has not yet been implemented")
		})
	}
	if api.SpacesListAllDomainsForSpaceDeprecatedHandler == nil {
		api.SpacesListAllDomainsForSpaceDeprecatedHandler = spaces.ListAllDomainsForSpaceDeprecatedHandlerFunc(func(params spaces.ListAllDomainsForSpaceDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllDomainsForSpaceDeprecated has not yet been implemented")
		})
	}
	if api.SpacesListAllEventsForSpaceHandler == nil {
		api.SpacesListAllEventsForSpaceHandler = spaces.ListAllEventsForSpaceHandlerFunc(func(params spaces.ListAllEventsForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllEventsForSpace has not yet been implemented")
		})
	}
	if api.UsersListAllManagedOrganizationsForUserHandler == nil {
		api.UsersListAllManagedOrganizationsForUserHandler = users.ListAllManagedOrganizationsForUserHandlerFunc(func(params users.ListAllManagedOrganizationsForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllManagedOrganizationsForUser has not yet been implemented")
		})
	}
	if api.UsersListAllManagedSpacesForUserHandler == nil {
		api.UsersListAllManagedSpacesForUserHandler = users.ListAllManagedSpacesForUserHandlerFunc(func(params users.ListAllManagedSpacesForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllManagedSpacesForUser has not yet been implemented")
		})
	}
	if api.OrganizationsListAllManagersForOrganizationHandler == nil {
		api.OrganizationsListAllManagersForOrganizationHandler = organizations.ListAllManagersForOrganizationHandlerFunc(func(params organizations.ListAllManagersForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllManagersForOrganization has not yet been implemented")
		})
	}
	if api.SpacesListAllManagersForSpaceHandler == nil {
		api.SpacesListAllManagersForSpaceHandler = spaces.ListAllManagersForSpaceHandlerFunc(func(params spaces.ListAllManagersForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllManagersForSpace has not yet been implemented")
		})
	}

	api.ResourceMatchListAllMatchingResourcesHandler = resource_match.ListAllMatchingResourcesHandlerFunc(func(params resource_match.ListAllMatchingResourcesParams) middleware.Responder {
		return resource_match.NewListAllMatchingResourcesOK().WithPayload(
			[]*models.ListAllMatchingResourcesResponse{},
		)
	})

	if api.OrganizationQuotaDefinitionsListAllOrganizationQuotaDefinitionsHandler == nil {
		api.OrganizationQuotaDefinitionsListAllOrganizationQuotaDefinitionsHandler = organization_quota_definitions.ListAllOrganizationQuotaDefinitionsHandlerFunc(func(params organization_quota_definitions.ListAllOrganizationQuotaDefinitionsParams) middleware.Responder {
			return middleware.NotImplemented("operation organization_quota_definitions.ListAllOrganizationQuotaDefinitions has not yet been implemented")
		})
	}

	api.OrganizationsListAllOrganizationsHandler = organizations.ListAllOrganizationsHandlerFunc(func(params organizations.ListAllOrganizationsParams) middleware.Responder {
		return organizations.NewListAllOrganizationsOK().WithPayload(
			&models.ListAllOrganizationsResponsePaged{
				Resources: []*models.ListAllOrganizationsResponseResource{
					&models.ListAllOrganizationsResponseResource{
						Metadata: &models.EntityMetadata{
							GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
						},
						Entity: &models.ListAllOrganizationsResponse{
							Name: "myorg",
						},
					},
				},
			},
		)
	})

	if api.UsersListAllOrganizationsForUserHandler == nil {
		api.UsersListAllOrganizationsForUserHandler = users.ListAllOrganizationsForUserHandlerFunc(func(params users.ListAllOrganizationsForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllOrganizationsForUser has not yet been implemented")
		})
	}
	if api.OrganizationsListAllPrivateDomainsForOrganizationHandler == nil {
		api.OrganizationsListAllPrivateDomainsForOrganizationHandler = organizations.ListAllPrivateDomainsForOrganizationHandlerFunc(func(params organizations.ListAllPrivateDomainsForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllPrivateDomainsForOrganization has not yet been implemented")
		})
	}

	api.RoutesListAllRoutesHandler = routes.ListAllRoutesHandlerFunc(func(params routes.ListAllRoutesParams) middleware.Responder {
		return routes.NewListAllRoutesOK().WithPayload(
			&models.ListAllRoutesResponsePaged{
				Resources: []*models.ListAllRoutesResponseResource{
					&models.ListAllRoutesResponseResource{
						Metadata: &models.EntityMetadata{
							GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
						},
						Entity: &models.ListAllRoutesResponse{
							Host: "myapp",
						},
					},
				},
			},
		)
	})

	if api.AppsListAllRoutesForAppHandler == nil {
		api.AppsListAllRoutesForAppHandler = apps.ListAllRoutesForAppHandlerFunc(func(params apps.ListAllRoutesForAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.ListAllRoutesForApp has not yet been implemented")
		})
	}
	if api.SpacesListAllRoutesForSpaceHandler == nil {
		api.SpacesListAllRoutesForSpaceHandler = spaces.ListAllRoutesForSpaceHandlerFunc(func(params spaces.ListAllRoutesForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllRoutesForSpace has not yet been implemented")
		})
	}
	if api.SecurityGroupsListAllSecurityGroupsHandler == nil {
		api.SecurityGroupsListAllSecurityGroupsHandler = security_groups.ListAllSecurityGroupsHandlerFunc(func(params security_groups.ListAllSecurityGroupsParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.ListAllSecurityGroups has not yet been implemented")
		})
	}
	if api.SpacesListAllSecurityGroupsForSpaceHandler == nil {
		api.SpacesListAllSecurityGroupsForSpaceHandler = spaces.ListAllSecurityGroupsForSpaceHandlerFunc(func(params spaces.ListAllSecurityGroupsForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllSecurityGroupsForSpace has not yet been implemented")
		})
	}
	if api.ServiceBindingsListAllServiceBindingsHandler == nil {
		api.ServiceBindingsListAllServiceBindingsHandler = service_bindings.ListAllServiceBindingsHandlerFunc(func(params service_bindings.ListAllServiceBindingsParams) middleware.Responder {
			return middleware.NotImplemented("operation service_bindings.ListAllServiceBindings has not yet been implemented")
		})
	}
	if api.AppsListAllServiceBindingsForAppHandler == nil {
		api.AppsListAllServiceBindingsForAppHandler = apps.ListAllServiceBindingsForAppHandlerFunc(func(params apps.ListAllServiceBindingsForAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.ListAllServiceBindingsForApp has not yet been implemented")
		})
	}
	if api.ServiceInstancesListAllServiceBindingsForServiceInstanceHandler == nil {
		api.ServiceInstancesListAllServiceBindingsForServiceInstanceHandler = service_instances.ListAllServiceBindingsForServiceInstanceHandlerFunc(func(params service_instances.ListAllServiceBindingsForServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.ListAllServiceBindingsForServiceInstance has not yet been implemented")
		})
	}
	if api.UserProvidedServiceInstancesListAllServiceBindingsForUserProvidedServiceInstanceHandler == nil {
		api.UserProvidedServiceInstancesListAllServiceBindingsForUserProvidedServiceInstanceHandler = user_provided_service_instances.ListAllServiceBindingsForUserProvidedServiceInstanceHandlerFunc(func(params user_provided_service_instances.ListAllServiceBindingsForUserProvidedServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation user_provided_service_instances.ListAllServiceBindingsForUserProvidedServiceInstance has not yet been implemented")
		})
	}
	if api.ServiceBrokersListAllServiceBrokersHandler == nil {
		api.ServiceBrokersListAllServiceBrokersHandler = service_brokers.ListAllServiceBrokersHandlerFunc(func(params service_brokers.ListAllServiceBrokersParams) middleware.Responder {
			return middleware.NotImplemented("operation service_brokers.ListAllServiceBrokers has not yet been implemented")
		})
	}
	if api.ServiceInstancesListAllServiceInstancesHandler == nil {
		api.ServiceInstancesListAllServiceInstancesHandler = service_instances.ListAllServiceInstancesHandlerFunc(func(params service_instances.ListAllServiceInstancesParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.ListAllServiceInstances has not yet been implemented")
		})
	}
	if api.SpacesListAllServiceInstancesForSpaceHandler == nil {
		api.SpacesListAllServiceInstancesForSpaceHandler = spaces.ListAllServiceInstancesForSpaceHandlerFunc(func(params spaces.ListAllServiceInstancesForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllServiceInstancesForSpace has not yet been implemented")
		})
	}
	if api.ServicePlanVisibilitiesListAllServicePlanVisibilitiesHandler == nil {
		api.ServicePlanVisibilitiesListAllServicePlanVisibilitiesHandler = service_plan_visibilities.ListAllServicePlanVisibilitiesHandlerFunc(func(params service_plan_visibilities.ListAllServicePlanVisibilitiesParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plan_visibilities.ListAllServicePlanVisibilities has not yet been implemented")
		})
	}
	if api.ServicePlansListAllServicePlansHandler == nil {
		api.ServicePlansListAllServicePlansHandler = service_plans.ListAllServicePlansHandlerFunc(func(params service_plans.ListAllServicePlansParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plans.ListAllServicePlans has not yet been implemented")
		})
	}
	if api.ServicesListAllServicePlansForServiceHandler == nil {
		api.ServicesListAllServicePlansForServiceHandler = services.ListAllServicePlansForServiceHandlerFunc(func(params services.ListAllServicePlansForServiceParams) middleware.Responder {
			return middleware.NotImplemented("operation services.ListAllServicePlansForService has not yet been implemented")
		})
	}
	if api.ServicesListAllServicesHandler == nil {
		api.ServicesListAllServicesHandler = services.ListAllServicesHandlerFunc(func(params services.ListAllServicesParams) middleware.Responder {
			return middleware.NotImplemented("operation services.ListAllServices has not yet been implemented")
		})
	}
	if api.OrganizationsListAllServicesForOrganizationHandler == nil {
		api.OrganizationsListAllServicesForOrganizationHandler = organizations.ListAllServicesForOrganizationHandlerFunc(func(params organizations.ListAllServicesForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllServicesForOrganization has not yet been implemented")
		})
	}
	if api.SpacesListAllServicesForSpaceHandler == nil {
		api.SpacesListAllServicesForSpaceHandler = spaces.ListAllServicesForSpaceHandlerFunc(func(params spaces.ListAllServicesForSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllServicesForSpace has not yet been implemented")
		})
	}
	if api.SharedDomainsListAllSharedDomainsHandler == nil {
		api.SharedDomainsListAllSharedDomainsHandler = shared_domains.ListAllSharedDomainsHandlerFunc(func(params shared_domains.ListAllSharedDomainsParams) middleware.Responder {
			return middleware.NotImplemented("operation shared_domains.ListAllSharedDomains has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsListAllSpaceQuotaDefinitionsHandler == nil {
		api.SpaceQuotaDefinitionsListAllSpaceQuotaDefinitionsHandler = space_quota_definitions.ListAllSpaceQuotaDefinitionsHandlerFunc(func(params space_quota_definitions.ListAllSpaceQuotaDefinitionsParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.ListAllSpaceQuotaDefinitions has not yet been implemented")
		})
	}
	if api.OrganizationsListAllSpaceQuotaDefinitionsForOrganizationHandler == nil {
		api.OrganizationsListAllSpaceQuotaDefinitionsForOrganizationHandler = organizations.ListAllSpaceQuotaDefinitionsForOrganizationHandlerFunc(func(params organizations.ListAllSpaceQuotaDefinitionsForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllSpaceQuotaDefinitionsForOrganization has not yet been implemented")
		})
	}
	if api.SpacesListAllSpacesHandler == nil {
		api.SpacesListAllSpacesHandler = spaces.ListAllSpacesHandlerFunc(func(params spaces.ListAllSpacesParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.ListAllSpaces has not yet been implemented")
		})
	}
	if api.DomainsDeprecatedListAllSpacesForDomainDeprecatedHandler == nil {
		api.DomainsDeprecatedListAllSpacesForDomainDeprecatedHandler = domains_deprecated.ListAllSpacesForDomainDeprecatedHandlerFunc(func(params domains_deprecated.ListAllSpacesForDomainDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation domains_deprecated.ListAllSpacesForDomainDeprecated has not yet been implemented")
		})
	}

	api.OrganizationsListAllSpacesForOrganizationHandler = organizations.ListAllSpacesForOrganizationHandlerFunc(func(params organizations.ListAllSpacesForOrganizationParams) middleware.Responder {
		return organizations.NewListAllSpacesForOrganizationOK().WithPayload(
			&models.ListAllSpacesForOrganizationResponsePaged{
				Resources: []*models.ListAllSpacesForOrganizationResponseResource{
					&models.ListAllSpacesForOrganizationResponseResource{
						Metadata: &models.EntityMetadata{
							GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
						},
						Entity: &models.ListAllSpacesForOrganizationResponse{
							Name: "myspace",
						},
					},
				},
			},
		)
	})

	if api.SecurityGroupsListAllSpacesForSecurityGroupHandler == nil {
		api.SecurityGroupsListAllSpacesForSecurityGroupHandler = security_groups.ListAllSpacesForSecurityGroupHandlerFunc(func(params security_groups.ListAllSpacesForSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.ListAllSpacesForSecurityGroup has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsListAllSpacesForSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsListAllSpacesForSpaceQuotaDefinitionHandler = space_quota_definitions.ListAllSpacesForSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.ListAllSpacesForSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.ListAllSpacesForSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.UsersListAllSpacesForUserHandler == nil {
		api.UsersListAllSpacesForUserHandler = users.ListAllSpacesForUserHandlerFunc(func(params users.ListAllSpacesForUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllSpacesForUser has not yet been implemented")
		})
	}

	api.StacksListAllStacksHandler = stacks.ListAllStacksHandlerFunc(func(params stacks.ListAllStacksParams) middleware.Responder {
		return stacks.NewListAllStacksOK().WithPayload(
			&models.ListAllStacksResponsePaged{
				Resources: []*models.ListAllStacksResponseResource{
					&models.ListAllStacksResponseResource{
						Metadata: &models.EntityMetadata{
							GUID: "f1792c5c-eb04-47d3-8af3-5139f87ebdf9",
						},
						Entity: &models.ListAllStacksResponse{
							Name: "carrierstack",
						},
					},
				},
			},
		)
	})

	if api.UserProvidedServiceInstancesListAllUserProvidedServiceInstancesHandler == nil {
		api.UserProvidedServiceInstancesListAllUserProvidedServiceInstancesHandler = user_provided_service_instances.ListAllUserProvidedServiceInstancesHandlerFunc(func(params user_provided_service_instances.ListAllUserProvidedServiceInstancesParams) middleware.Responder {
			return middleware.NotImplemented("operation user_provided_service_instances.ListAllUserProvidedServiceInstances has not yet been implemented")
		})
	}
	if api.UsersListAllUsersHandler == nil {
		api.UsersListAllUsersHandler = users.ListAllUsersHandlerFunc(func(params users.ListAllUsersParams) middleware.Responder {
			return middleware.NotImplemented("operation users.ListAllUsers has not yet been implemented")
		})
	}
	if api.OrganizationsListAllUsersForOrganizationHandler == nil {
		api.OrganizationsListAllUsersForOrganizationHandler = organizations.ListAllUsersForOrganizationHandlerFunc(func(params organizations.ListAllUsersForOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.ListAllUsersForOrganization has not yet been implemented")
		})
	}
	if api.EventsListServiceBrokerDeleteEventsExperimentalHandler == nil {
		api.EventsListServiceBrokerDeleteEventsExperimentalHandler = events.ListServiceBrokerDeleteEventsExperimentalHandlerFunc(func(params events.ListServiceBrokerDeleteEventsExperimentalParams) middleware.Responder {
			return middleware.NotImplemented("operation events.ListServiceBrokerDeleteEventsExperimental has not yet been implemented")
		})
	}
	if api.ServiceUsageEventsExperimentalListServiceUsageEventsHandler == nil {
		api.ServiceUsageEventsExperimentalListServiceUsageEventsHandler = service_usage_events_experimental.ListServiceUsageEventsHandlerFunc(func(params service_usage_events_experimental.ListServiceUsageEventsParams) middleware.Responder {
			return middleware.NotImplemented("operation service_usage_events_experimental.ListServiceUsageEvents has not yet been implemented")
		})
	}
	if api.BuildpacksLockOrUnlockBuildpackHandler == nil {
		api.BuildpacksLockOrUnlockBuildpackHandler = buildpacks.LockOrUnlockBuildpackHandlerFunc(func(params buildpacks.LockOrUnlockBuildpackParams) middleware.Responder {
			return middleware.NotImplemented("operation buildpacks.LockOrUnlockBuildpack has not yet been implemented")
		})
	}
	if api.ServiceInstancesMigrateServiceInstancesFromOneServicePlanToAnotherServicePlanExperimentalHandler == nil {
		api.ServiceInstancesMigrateServiceInstancesFromOneServicePlanToAnotherServicePlanExperimentalHandler = service_instances.MigrateServiceInstancesFromOneServicePlanToAnotherServicePlanExperimentalHandlerFunc(func(params service_instances.MigrateServiceInstancesFromOneServicePlanToAnotherServicePlanExperimentalParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.MigrateServiceInstancesFromOneServicePlanToAnotherServicePlanExperimental has not yet been implemented")
		})
	}
	if api.AppUsageEventsPurgeAndReseedAppUsageEventsHandler == nil {
		api.AppUsageEventsPurgeAndReseedAppUsageEventsHandler = app_usage_events.PurgeAndReseedAppUsageEventsHandlerFunc(func(params app_usage_events.PurgeAndReseedAppUsageEventsParams) middleware.Responder {
			return middleware.NotImplemented("operation app_usage_events.PurgeAndReseedAppUsageEvents has not yet been implemented")
		})
	}
	if api.ServiceUsageEventsExperimentalPurgeAndReseedServiceUsageEventsHandler == nil {
		api.ServiceUsageEventsExperimentalPurgeAndReseedServiceUsageEventsHandler = service_usage_events_experimental.PurgeAndReseedServiceUsageEventsHandlerFunc(func(params service_usage_events_experimental.PurgeAndReseedServiceUsageEventsParams) middleware.Responder {
			return middleware.NotImplemented("operation service_usage_events_experimental.PurgeAndReseedServiceUsageEvents has not yet been implemented")
		})
	}
	if api.RoutesRemoveAppFromRouteHandler == nil {
		api.RoutesRemoveAppFromRouteHandler = routes.RemoveAppFromRouteHandlerFunc(func(params routes.RemoveAppFromRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.RemoveAppFromRoute has not yet been implemented")
		})
	}
	if api.UsersRemoveAuditedOrganizationFromUserHandler == nil {
		api.UsersRemoveAuditedOrganizationFromUserHandler = users.RemoveAuditedOrganizationFromUserHandlerFunc(func(params users.RemoveAuditedOrganizationFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveAuditedOrganizationFromUser has not yet been implemented")
		})
	}
	if api.UsersRemoveAuditedSpaceFromUserHandler == nil {
		api.UsersRemoveAuditedSpaceFromUserHandler = users.RemoveAuditedSpaceFromUserHandlerFunc(func(params users.RemoveAuditedSpaceFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveAuditedSpaceFromUser has not yet been implemented")
		})
	}
	if api.OrganizationsRemoveAuditorFromOrganizationHandler == nil {
		api.OrganizationsRemoveAuditorFromOrganizationHandler = organizations.RemoveAuditorFromOrganizationHandlerFunc(func(params organizations.RemoveAuditorFromOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.RemoveAuditorFromOrganization has not yet been implemented")
		})
	}
	if api.SpacesRemoveAuditorFromSpaceHandler == nil {
		api.SpacesRemoveAuditorFromSpaceHandler = spaces.RemoveAuditorFromSpaceHandlerFunc(func(params spaces.RemoveAuditorFromSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.RemoveAuditorFromSpace has not yet been implemented")
		})
	}
	if api.UsersRemoveBillingManagedOrganizationFromUserHandler == nil {
		api.UsersRemoveBillingManagedOrganizationFromUserHandler = users.RemoveBillingManagedOrganizationFromUserHandlerFunc(func(params users.RemoveBillingManagedOrganizationFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveBillingManagedOrganizationFromUser has not yet been implemented")
		})
	}
	if api.OrganizationsRemoveBillingManagerFromOrganizationHandler == nil {
		api.OrganizationsRemoveBillingManagerFromOrganizationHandler = organizations.RemoveBillingManagerFromOrganizationHandlerFunc(func(params organizations.RemoveBillingManagerFromOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.RemoveBillingManagerFromOrganization has not yet been implemented")
		})
	}
	if api.SpacesRemoveDeveloperFromSpaceHandler == nil {
		api.SpacesRemoveDeveloperFromSpaceHandler = spaces.RemoveDeveloperFromSpaceHandlerFunc(func(params spaces.RemoveDeveloperFromSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.RemoveDeveloperFromSpace has not yet been implemented")
		})
	}
	if api.UsersRemoveManagedOrganizationFromUserHandler == nil {
		api.UsersRemoveManagedOrganizationFromUserHandler = users.RemoveManagedOrganizationFromUserHandlerFunc(func(params users.RemoveManagedOrganizationFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveManagedOrganizationFromUser has not yet been implemented")
		})
	}
	if api.UsersRemoveManagedSpaceFromUserHandler == nil {
		api.UsersRemoveManagedSpaceFromUserHandler = users.RemoveManagedSpaceFromUserHandlerFunc(func(params users.RemoveManagedSpaceFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveManagedSpaceFromUser has not yet been implemented")
		})
	}
	if api.OrganizationsRemoveManagerFromOrganizationHandler == nil {
		api.OrganizationsRemoveManagerFromOrganizationHandler = organizations.RemoveManagerFromOrganizationHandlerFunc(func(params organizations.RemoveManagerFromOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.RemoveManagerFromOrganization has not yet been implemented")
		})
	}
	if api.SpacesRemoveManagerFromSpaceHandler == nil {
		api.SpacesRemoveManagerFromSpaceHandler = spaces.RemoveManagerFromSpaceHandlerFunc(func(params spaces.RemoveManagerFromSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.RemoveManagerFromSpace has not yet been implemented")
		})
	}
	if api.UsersRemoveOrganizationFromUserHandler == nil {
		api.UsersRemoveOrganizationFromUserHandler = users.RemoveOrganizationFromUserHandlerFunc(func(params users.RemoveOrganizationFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveOrganizationFromUser has not yet been implemented")
		})
	}
	if api.AppsRemoveRouteFromAppHandler == nil {
		api.AppsRemoveRouteFromAppHandler = apps.RemoveRouteFromAppHandlerFunc(func(params apps.RemoveRouteFromAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.RemoveRouteFromApp has not yet been implemented")
		})
	}
	if api.SpacesRemoveSecurityGroupFromSpaceHandler == nil {
		api.SpacesRemoveSecurityGroupFromSpaceHandler = spaces.RemoveSecurityGroupFromSpaceHandlerFunc(func(params spaces.RemoveSecurityGroupFromSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.RemoveSecurityGroupFromSpace has not yet been implemented")
		})
	}
	if api.AppsRemoveServiceBindingFromAppHandler == nil {
		api.AppsRemoveServiceBindingFromAppHandler = apps.RemoveServiceBindingFromAppHandlerFunc(func(params apps.RemoveServiceBindingFromAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.RemoveServiceBindingFromApp has not yet been implemented")
		})
	}
	if api.SecurityGroupsRemoveSpaceFromSecurityGroupHandler == nil {
		api.SecurityGroupsRemoveSpaceFromSecurityGroupHandler = security_groups.RemoveSpaceFromSecurityGroupHandlerFunc(func(params security_groups.RemoveSpaceFromSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.RemoveSpaceFromSecurityGroup has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsRemoveSpaceFromSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsRemoveSpaceFromSpaceQuotaDefinitionHandler = space_quota_definitions.RemoveSpaceFromSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.RemoveSpaceFromSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.RemoveSpaceFromSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.UsersRemoveSpaceFromUserHandler == nil {
		api.UsersRemoveSpaceFromUserHandler = users.RemoveSpaceFromUserHandlerFunc(func(params users.RemoveSpaceFromUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RemoveSpaceFromUser has not yet been implemented")
		})
	}
	if api.OrganizationsRemoveUserFromOrganizationHandler == nil {
		api.OrganizationsRemoveUserFromOrganizationHandler = organizations.RemoveUserFromOrganizationHandlerFunc(func(params organizations.RemoveUserFromOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.RemoveUserFromOrganization has not yet been implemented")
		})
	}
	if api.SecurityGroupRunningDefaultsRemovingSecurityGroupAsDefaultForRunningAppsHandler == nil {
		api.SecurityGroupRunningDefaultsRemovingSecurityGroupAsDefaultForRunningAppsHandler = security_group_running_defaults.RemovingSecurityGroupAsDefaultForRunningAppsHandlerFunc(func(params security_group_running_defaults.RemovingSecurityGroupAsDefaultForRunningAppsParams) middleware.Responder {
			return middleware.NotImplemented("operation security_group_running_defaults.RemovingSecurityGroupAsDefaultForRunningApps has not yet been implemented")
		})
	}
	if api.SecurityGroupStagingDefaultsRemovingSecurityGroupAsDefaultForStagingHandler == nil {
		api.SecurityGroupStagingDefaultsRemovingSecurityGroupAsDefaultForStagingHandler = security_group_staging_defaults.RemovingSecurityGroupAsDefaultForStagingHandlerFunc(func(params security_group_staging_defaults.RemovingSecurityGroupAsDefaultForStagingParams) middleware.Responder {
			return middleware.NotImplemented("operation security_group_staging_defaults.RemovingSecurityGroupAsDefaultForStaging has not yet been implemented")
		})
	}
	if api.AppsRestageAppHandler == nil {
		api.AppsRestageAppHandler = apps.RestageAppHandlerFunc(func(params apps.RestageAppParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.RestageApp has not yet been implemented")
		})
	}

	api.AppsRetrieveAppHandler = apps.RetrieveAppHandlerFunc(func(params apps.RetrieveAppParams) middleware.Responder {
		pushInfo := gitea.PushInfo{
			AppName: "myapp",
		}

		state, err := gitea.StagingStatus(pushInfo)
		if err != nil {
			return middleware.Error(504, err)
		}

		return apps.NewRetrieveAppOK().WithPayload(
			&models.RetrieveAppResponseResource{
				Metadata: &models.EntityMetadata{
					GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
					URL:  "/v2/apps/4feb778f-9d26-4332-9868-e03715fae08f",
				},
				Entity: &models.RetrieveAppResponse{
					Name:         "foo",
					State:        "STARTED",
					PackageState: state,
				},
			},
		)
	})

	if api.AppUsageEventsRetrieveAppUsageEventHandler == nil {
		api.AppUsageEventsRetrieveAppUsageEventHandler = app_usage_events.RetrieveAppUsageEventHandlerFunc(func(params app_usage_events.RetrieveAppUsageEventParams) middleware.Responder {
			return middleware.NotImplemented("operation app_usage_events.RetrieveAppUsageEvent has not yet been implemented")
		})
	}
	if api.BuildpacksRetrieveBuildpackHandler == nil {
		api.BuildpacksRetrieveBuildpackHandler = buildpacks.RetrieveBuildpackHandlerFunc(func(params buildpacks.RetrieveBuildpackParams) middleware.Responder {
			return middleware.NotImplemented("operation buildpacks.RetrieveBuildpack has not yet been implemented")
		})
	}
	if api.DomainsDeprecatedRetrieveDomainDeprecatedHandler == nil {
		api.DomainsDeprecatedRetrieveDomainDeprecatedHandler = domains_deprecated.RetrieveDomainDeprecatedHandlerFunc(func(params domains_deprecated.RetrieveDomainDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation domains_deprecated.RetrieveDomainDeprecated has not yet been implemented")
		})
	}
	if api.EventsRetrieveEventHandler == nil {
		api.EventsRetrieveEventHandler = events.RetrieveEventHandlerFunc(func(params events.RetrieveEventParams) middleware.Responder {
			return middleware.NotImplemented("operation events.RetrieveEvent has not yet been implemented")
		})
	}
	if api.FilesRetrieveFileHandler == nil {
		api.FilesRetrieveFileHandler = files.RetrieveFileHandlerFunc(func(params files.RetrieveFileParams) middleware.Responder {
			return middleware.NotImplemented("operation files.RetrieveFile has not yet been implemented")
		})
	}
	if api.JobsRetrieveJobThatWasSuccessfulHandler == nil {
		api.JobsRetrieveJobThatWasSuccessfulHandler = jobs.RetrieveJobThatWasSuccessfulHandlerFunc(func(params jobs.RetrieveJobThatWasSuccessfulParams) middleware.Responder {
			return middleware.NotImplemented("operation jobs.RetrieveJobThatWasSuccessful has not yet been implemented")
		})
	}
	if api.OrganizationsRetrieveOrganizationHandler == nil {
		api.OrganizationsRetrieveOrganizationHandler = organizations.RetrieveOrganizationHandlerFunc(func(params organizations.RetrieveOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.RetrieveOrganization has not yet been implemented")
		})
	}
	if api.OrganizationQuotaDefinitionsRetrieveOrganizationQuotaDefinitionHandler == nil {
		api.OrganizationQuotaDefinitionsRetrieveOrganizationQuotaDefinitionHandler = organization_quota_definitions.RetrieveOrganizationQuotaDefinitionHandlerFunc(func(params organization_quota_definitions.RetrieveOrganizationQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation organization_quota_definitions.RetrieveOrganizationQuotaDefinition has not yet been implemented")
		})
	}
	if api.PrivateDomainsRetrievePrivateDomainHandler == nil {
		api.PrivateDomainsRetrievePrivateDomainHandler = private_domains.RetrievePrivateDomainHandlerFunc(func(params private_domains.RetrievePrivateDomainParams) middleware.Responder {
			return middleware.NotImplemented("operation private_domains.RetrievePrivateDomain has not yet been implemented")
		})
	}
	if api.RoutesRetrieveRouteHandler == nil {
		api.RoutesRetrieveRouteHandler = routes.RetrieveRouteHandlerFunc(func(params routes.RetrieveRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.RetrieveRoute has not yet been implemented")
		})
	}
	if api.SecurityGroupsRetrieveSecurityGroupHandler == nil {
		api.SecurityGroupsRetrieveSecurityGroupHandler = security_groups.RetrieveSecurityGroupHandlerFunc(func(params security_groups.RetrieveSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.RetrieveSecurityGroup has not yet been implemented")
		})
	}
	if api.ServicesRetrieveServiceHandler == nil {
		api.ServicesRetrieveServiceHandler = services.RetrieveServiceHandlerFunc(func(params services.RetrieveServiceParams) middleware.Responder {
			return middleware.NotImplemented("operation services.RetrieveService has not yet been implemented")
		})
	}
	if api.ServiceAuthTokensDeprecatedRetrieveServiceAuthTokenDeprecatedHandler == nil {
		api.ServiceAuthTokensDeprecatedRetrieveServiceAuthTokenDeprecatedHandler = service_auth_tokens_deprecated.RetrieveServiceAuthTokenDeprecatedHandlerFunc(func(params service_auth_tokens_deprecated.RetrieveServiceAuthTokenDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation service_auth_tokens_deprecated.RetrieveServiceAuthTokenDeprecated has not yet been implemented")
		})
	}
	if api.ServiceBindingsRetrieveServiceBindingHandler == nil {
		api.ServiceBindingsRetrieveServiceBindingHandler = service_bindings.RetrieveServiceBindingHandlerFunc(func(params service_bindings.RetrieveServiceBindingParams) middleware.Responder {
			return middleware.NotImplemented("operation service_bindings.RetrieveServiceBinding has not yet been implemented")
		})
	}
	if api.ServiceBrokersRetrieveServiceBrokerHandler == nil {
		api.ServiceBrokersRetrieveServiceBrokerHandler = service_brokers.RetrieveServiceBrokerHandlerFunc(func(params service_brokers.RetrieveServiceBrokerParams) middleware.Responder {
			return middleware.NotImplemented("operation service_brokers.RetrieveServiceBroker has not yet been implemented")
		})
	}
	if api.ServiceInstancesRetrieveServiceInstanceHandler == nil {
		api.ServiceInstancesRetrieveServiceInstanceHandler = service_instances.RetrieveServiceInstanceHandlerFunc(func(params service_instances.RetrieveServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.RetrieveServiceInstance has not yet been implemented")
		})
	}
	if api.ServicePlansRetrieveServicePlanHandler == nil {
		api.ServicePlansRetrieveServicePlanHandler = service_plans.RetrieveServicePlanHandlerFunc(func(params service_plans.RetrieveServicePlanParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plans.RetrieveServicePlan has not yet been implemented")
		})
	}
	if api.ServicePlanVisibilitiesRetrieveServicePlanVisibilityHandler == nil {
		api.ServicePlanVisibilitiesRetrieveServicePlanVisibilityHandler = service_plan_visibilities.RetrieveServicePlanVisibilityHandlerFunc(func(params service_plan_visibilities.RetrieveServicePlanVisibilityParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plan_visibilities.RetrieveServicePlanVisibility has not yet been implemented")
		})
	}
	if api.ServiceUsageEventsExperimentalRetrieveServiceUsageEventHandler == nil {
		api.ServiceUsageEventsExperimentalRetrieveServiceUsageEventHandler = service_usage_events_experimental.RetrieveServiceUsageEventHandlerFunc(func(params service_usage_events_experimental.RetrieveServiceUsageEventParams) middleware.Responder {
			return middleware.NotImplemented("operation service_usage_events_experimental.RetrieveServiceUsageEvent has not yet been implemented")
		})
	}
	if api.SharedDomainsRetrieveSharedDomainHandler == nil {
		api.SharedDomainsRetrieveSharedDomainHandler = shared_domains.RetrieveSharedDomainHandlerFunc(func(params shared_domains.RetrieveSharedDomainParams) middleware.Responder {
			return middleware.NotImplemented("operation shared_domains.RetrieveSharedDomain has not yet been implemented")
		})
	}
	if api.SpacesRetrieveSpaceHandler == nil {
		api.SpacesRetrieveSpaceHandler = spaces.RetrieveSpaceHandlerFunc(func(params spaces.RetrieveSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.RetrieveSpace has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsRetrieveSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsRetrieveSpaceQuotaDefinitionHandler = space_quota_definitions.RetrieveSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.RetrieveSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.RetrieveSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.StacksRetrieveStackHandler == nil {
		api.StacksRetrieveStackHandler = stacks.RetrieveStackHandlerFunc(func(params stacks.RetrieveStackParams) middleware.Responder {
			return middleware.NotImplemented("operation stacks.RetrieveStack has not yet been implemented")
		})
	}
	if api.UsersRetrieveUserHandler == nil {
		api.UsersRetrieveUserHandler = users.RetrieveUserHandlerFunc(func(params users.RetrieveUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.RetrieveUser has not yet been implemented")
		})
	}
	if api.UserProvidedServiceInstancesRetrieveUserProvidedServiceInstanceHandler == nil {
		api.UserProvidedServiceInstancesRetrieveUserProvidedServiceInstanceHandler = user_provided_service_instances.RetrieveUserProvidedServiceInstanceHandlerFunc(func(params user_provided_service_instances.RetrieveUserProvidedServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation user_provided_service_instances.RetrieveUserProvidedServiceInstance has not yet been implemented")
		})
	}
	if api.OrganizationsRetrievingOrganizationMemoryUsageHandler == nil {
		api.OrganizationsRetrievingOrganizationMemoryUsageHandler = organizations.RetrievingOrganizationMemoryUsageHandlerFunc(func(params organizations.RetrievingOrganizationMemoryUsageParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.RetrievingOrganizationMemoryUsage has not yet been implemented")
		})
	}
	if api.ServiceInstancesRetrievingPermissionsOnServiceInstanceHandler == nil {
		api.ServiceInstancesRetrievingPermissionsOnServiceInstanceHandler = service_instances.RetrievingPermissionsOnServiceInstanceHandlerFunc(func(params service_instances.RetrievingPermissionsOnServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.RetrievingPermissionsOnServiceInstance has not yet been implemented")
		})
	}
	if api.SecurityGroupRunningDefaultsReturnSecurityGroupsUsedForRunningAppsHandler == nil {
		api.SecurityGroupRunningDefaultsReturnSecurityGroupsUsedForRunningAppsHandler = security_group_running_defaults.ReturnSecurityGroupsUsedForRunningAppsHandlerFunc(func(params security_group_running_defaults.ReturnSecurityGroupsUsedForRunningAppsParams) middleware.Responder {
			return middleware.NotImplemented("operation security_group_running_defaults.ReturnSecurityGroupsUsedForRunningApps has not yet been implemented")
		})
	}
	if api.SecurityGroupStagingDefaultsReturnSecurityGroupsUsedForStagingHandler == nil {
		api.SecurityGroupStagingDefaultsReturnSecurityGroupsUsedForStagingHandler = security_group_staging_defaults.ReturnSecurityGroupsUsedForStagingHandlerFunc(func(params security_group_staging_defaults.ReturnSecurityGroupsUsedForStagingParams) middleware.Responder {
			return middleware.NotImplemented("operation security_group_staging_defaults.ReturnSecurityGroupsUsedForStaging has not yet been implemented")
		})
	}
	if api.FeatureFlagsSetFeatureFlagHandler == nil {
		api.FeatureFlagsSetFeatureFlagHandler = feature_flags.SetFeatureFlagHandlerFunc(func(params feature_flags.SetFeatureFlagParams) middleware.Responder {
			return middleware.NotImplemented("operation feature_flags.SetFeatureFlag has not yet been implemented")
		})
	}
	if api.SecurityGroupRunningDefaultsSetSecurityGroupAsDefaultForRunningAppsHandler == nil {
		api.SecurityGroupRunningDefaultsSetSecurityGroupAsDefaultForRunningAppsHandler = security_group_running_defaults.SetSecurityGroupAsDefaultForRunningAppsHandlerFunc(func(params security_group_running_defaults.SetSecurityGroupAsDefaultForRunningAppsParams) middleware.Responder {
			return middleware.NotImplemented("operation security_group_running_defaults.SetSecurityGroupAsDefaultForRunningApps has not yet been implemented")
		})
	}
	if api.SecurityGroupStagingDefaultsSetSecurityGroupAsDefaultForStagingHandler == nil {
		api.SecurityGroupStagingDefaultsSetSecurityGroupAsDefaultForStagingHandler = security_group_staging_defaults.SetSecurityGroupAsDefaultForStagingHandlerFunc(func(params security_group_staging_defaults.SetSecurityGroupAsDefaultForStagingParams) middleware.Responder {
			return middleware.NotImplemented("operation security_group_staging_defaults.SetSecurityGroupAsDefaultForStaging has not yet been implemented")
		})
	}
	if api.AppsTerminateRunningAppInstanceAtGivenIndexHandler == nil {
		api.AppsTerminateRunningAppInstanceAtGivenIndexHandler = apps.TerminateRunningAppInstanceAtGivenIndexHandlerFunc(func(params apps.TerminateRunningAppInstanceAtGivenIndexParams) middleware.Responder {
			return middleware.NotImplemented("operation apps.TerminateRunningAppInstanceAtGivenIndex has not yet been implemented")
		})
	}

	api.AppsUpdateAppHandler = apps.UpdateAppHandlerFunc(func(params apps.UpdateAppParams) middleware.Responder {

		pushInfo := gitea.PushInfo{
			AppName:  params.Value.Name,
			Username: "dev",
			Password: "changeme",
			Target:   "10.1.5.5.nip.io",
		}

		err := gitea.CreateRepo(pushInfo)
		if err != nil {
			fmt.Println(err)
			return middleware.Error(504, err)
		}

		return apps.NewUpdateAppCreated().WithPayload(
			&models.UpdateAppResponseResource{
				Metadata: &models.EntityMetadata{
					GUID: "1811064d-d890-410b-98f9-01aedea105d6",
				},
				Entity: &models.UpdateAppResponse{
					Name:      "app1",
					Instances: 1,
				},
			},
		)
	})

	if api.EnvironmentVariableGroupsUpdateContentsOfRunningEnvironmentVariableGroupHandler == nil {
		api.EnvironmentVariableGroupsUpdateContentsOfRunningEnvironmentVariableGroupHandler = environment_variable_groups.UpdateContentsOfRunningEnvironmentVariableGroupHandlerFunc(func(params environment_variable_groups.UpdateContentsOfRunningEnvironmentVariableGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation environment_variable_groups.UpdateContentsOfRunningEnvironmentVariableGroup has not yet been implemented")
		})
	}
	if api.EnvironmentVariableGroupsUpdateContentsOfStagingEnvironmentVariableGroupHandler == nil {
		api.EnvironmentVariableGroupsUpdateContentsOfStagingEnvironmentVariableGroupHandler = environment_variable_groups.UpdateContentsOfStagingEnvironmentVariableGroupHandlerFunc(func(params environment_variable_groups.UpdateContentsOfStagingEnvironmentVariableGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation environment_variable_groups.UpdateContentsOfStagingEnvironmentVariableGroup has not yet been implemented")
		})
	}
	if api.OrganizationsUpdateOrganizationHandler == nil {
		api.OrganizationsUpdateOrganizationHandler = organizations.UpdateOrganizationHandlerFunc(func(params organizations.UpdateOrganizationParams) middleware.Responder {
			return middleware.NotImplemented("operation organizations.UpdateOrganization has not yet been implemented")
		})
	}
	if api.OrganizationQuotaDefinitionsUpdateOrganizationQuotaDefinitionHandler == nil {
		api.OrganizationQuotaDefinitionsUpdateOrganizationQuotaDefinitionHandler = organization_quota_definitions.UpdateOrganizationQuotaDefinitionHandlerFunc(func(params organization_quota_definitions.UpdateOrganizationQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation organization_quota_definitions.UpdateOrganizationQuotaDefinition has not yet been implemented")
		})
	}
	if api.RoutesUpdateRouteHandler == nil {
		api.RoutesUpdateRouteHandler = routes.UpdateRouteHandlerFunc(func(params routes.UpdateRouteParams) middleware.Responder {
			return middleware.NotImplemented("operation routes.UpdateRoute has not yet been implemented")
		})
	}
	if api.SecurityGroupsUpdateSecurityGroupHandler == nil {
		api.SecurityGroupsUpdateSecurityGroupHandler = security_groups.UpdateSecurityGroupHandlerFunc(func(params security_groups.UpdateSecurityGroupParams) middleware.Responder {
			return middleware.NotImplemented("operation security_groups.UpdateSecurityGroup has not yet been implemented")
		})
	}
	if api.ServiceBrokersUpdateServiceBrokerHandler == nil {
		api.ServiceBrokersUpdateServiceBrokerHandler = service_brokers.UpdateServiceBrokerHandlerFunc(func(params service_brokers.UpdateServiceBrokerParams) middleware.Responder {
			return middleware.NotImplemented("operation service_brokers.UpdateServiceBroker has not yet been implemented")
		})
	}
	if api.ServicesUpdateServiceDeprecatedHandler == nil {
		api.ServicesUpdateServiceDeprecatedHandler = services.UpdateServiceDeprecatedHandlerFunc(func(params services.UpdateServiceDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation services.UpdateServiceDeprecated has not yet been implemented")
		})
	}
	if api.ServiceInstancesUpdateServiceInstanceHandler == nil {
		api.ServiceInstancesUpdateServiceInstanceHandler = service_instances.UpdateServiceInstanceHandlerFunc(func(params service_instances.UpdateServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation service_instances.UpdateServiceInstance has not yet been implemented")
		})
	}
	if api.ServicePlansUpdateServicePlanDeprecatedHandler == nil {
		api.ServicePlansUpdateServicePlanDeprecatedHandler = service_plans.UpdateServicePlanDeprecatedHandlerFunc(func(params service_plans.UpdateServicePlanDeprecatedParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plans.UpdateServicePlanDeprecated has not yet been implemented")
		})
	}
	if api.ServicePlanVisibilitiesUpdateServicePlanVisibilityHandler == nil {
		api.ServicePlanVisibilitiesUpdateServicePlanVisibilityHandler = service_plan_visibilities.UpdateServicePlanVisibilityHandlerFunc(func(params service_plan_visibilities.UpdateServicePlanVisibilityParams) middleware.Responder {
			return middleware.NotImplemented("operation service_plan_visibilities.UpdateServicePlanVisibility has not yet been implemented")
		})
	}
	if api.SpacesUpdateSpaceHandler == nil {
		api.SpacesUpdateSpaceHandler = spaces.UpdateSpaceHandlerFunc(func(params spaces.UpdateSpaceParams) middleware.Responder {
			return middleware.NotImplemented("operation spaces.UpdateSpace has not yet been implemented")
		})
	}
	if api.SpaceQuotaDefinitionsUpdateSpaceQuotaDefinitionHandler == nil {
		api.SpaceQuotaDefinitionsUpdateSpaceQuotaDefinitionHandler = space_quota_definitions.UpdateSpaceQuotaDefinitionHandlerFunc(func(params space_quota_definitions.UpdateSpaceQuotaDefinitionParams) middleware.Responder {
			return middleware.NotImplemented("operation space_quota_definitions.UpdateSpaceQuotaDefinition has not yet been implemented")
		})
	}
	if api.UsersUpdateUserHandler == nil {
		api.UsersUpdateUserHandler = users.UpdateUserHandlerFunc(func(params users.UpdateUserParams) middleware.Responder {
			return middleware.NotImplemented("operation users.UpdateUser has not yet been implemented")
		})
	}
	if api.UserProvidedServiceInstancesUpdateUserProvidedServiceInstanceHandler == nil {
		api.UserProvidedServiceInstancesUpdateUserProvidedServiceInstanceHandler = user_provided_service_instances.UpdateUserProvidedServiceInstanceHandlerFunc(func(params user_provided_service_instances.UpdateUserProvidedServiceInstanceParams) middleware.Responder {
			return middleware.NotImplemented("operation user_provided_service_instances.UpdateUserProvidedServiceInstance has not yet been implemented")
		})
	}

	api.AppsUploadsBitsForAppHandler = apps.UploadsBitsForAppHandlerFunc(func(params apps.UploadsBitsForAppParams) middleware.Responder {
		// Unzip to a temp directory
		dirName, err := ioutil.TempDir("", "carrier-app")
		if err != nil {
			return middleware.Error(503, err)
		}
		defer os.RemoveAll(dirName)

		buff := bytes.NewBuffer([]byte{})
		size, err := io.Copy(buff, params.Application)
		if err != nil {
			return middleware.Error(503, err)
		}
		reader := bytes.NewReader(buff.Bytes())
		// Open a zip archive for reading.
		zipReader, err := zip.NewReader(reader, size)

		defer params.Application.Close()

		for _, f := range zipReader.File {
			// Store filename/path for returning and using later on
			fpath := filepath.Join(dirName, f.Name)

			// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
			if !strings.HasPrefix(fpath, filepath.Clean(dirName)+string(os.PathSeparator)) {
				return middleware.Error(504, fmt.Errorf("%s: illegal file path", fpath))
			}

			if f.FileInfo().IsDir() {
				// Make Folder
				os.MkdirAll(fpath, os.ModePerm)
				continue
			}

			// Make File
			if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
				return middleware.Error(504, err)
			}

			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return middleware.Error(504, err)
			}

			rc, err := f.Open()
			if err != nil {
				return middleware.Error(504, err)
			}

			_, err = io.Copy(outFile, rc)

			// Close the file without defer to close before next iteration of loop
			outFile.Close()
			rc.Close()

			if err != nil {
				return middleware.Error(504, err)
			}
		}

		// List files
		files, err := ioutil.ReadDir(dirName)
		if err != nil {
			fmt.Println(err)
		}
		for _, f := range files {
			fmt.Println(f.Name())
		}

		pushInfo := gitea.PushInfo{
			AppDir:        dirName,
			AppName:       "myapp",
			Username:      "dev",
			Password:      "changeme",
			Target:        "10.1.5.5.nip.io",
			ImageUser:     "viovanov",
			ImagePassword: "password1234!",
		}

		err = gitea.Push(pushInfo)
		if err != nil {
			return middleware.Error(504, err)
		}

		return apps.NewUploadsBitsForAppCreated().WithPayload(
			&models.UploadsBitsForAppResponseResource{
				Metadata: &models.EntityMetadata{
					GUID: "4feb778f-9d26-4332-9868-e03715fae08f",
				},
				Entity: &models.UploadsBitsForAppResponse{
					GUID:   "4feb778f-9d26-4332-9868-e03715fae08f",
					Status: "queued",
				},
			},
		)
	})

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix"
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
