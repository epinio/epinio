// Copyright © 2021 - 2026 KRUMWARE LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"
	"strings"

	"github.com/epinio/epinio/helpers"
	"github.com/epinio/epinio/internal/dex"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func oidcGroupLogger() *zap.SugaredLogger {
	if helpers.Logger != nil {
		return helpers.Logger.With("component", "oidcGroupRoles")
	}
	return zap.NewNop().Sugar()
}

// RolesFromProviderGroups returns Epinio roles matching Dex connector groups.
func RolesFromProviderGroups(ctx context.Context, oidcProvider *dex.OIDCProvider, providerID string, groups []string) Roles {
	logger := oidcGroupLogger()
	roles := Roles{}

	pg, err := oidcProvider.GetProviderGroups(providerID)
	if err != nil {
		logger.Infow(
			"error getting provider groups",
			"provider", providerID,
		)
		return roles
	}

	roleIDs := pg.GetRolesFromGroups(groups...)
	if len(roleIDs) == 0 {
		logger.Infow(
			"no matching groups found in provider groups",
			"provider", providerID,
			"providerGroups", pg,
			"groups", strings.Join(groups, ","),
		)
		return roles
	}

	for _, fullRoleID := range roleIDs {
		roleID, namespace := ParseRoleID(fullRoleID)

		userRole, found := EpinioRoles.FindByID(roleID)
		if !found {
			logger.Infow("role not found in Epinio", "roleID", roleID)
			continue
		}

		userRole.Namespace = namespace
		roles = append(roles, userRole)
	}

	return roles
}

// GetOrCreateUserByEmail returns the user with the matching email, or creates one.
// If some roles are provided then the user will be created or updated with those roles.
// If no roles are provided then we check for a default role. Existing roles are kept
// when no incoming groups were provided (manual assignment).
func GetOrCreateUserByEmail(ctx context.Context, email string, userRoles Roles) (User, error) {
	user := User{}
	var err error

	authService, err := NewAuthServiceFromContext(ctx)
	if err != nil {
		return user, errors.Wrap(err, "couldn't create auth service from context")
	}

	defaultRole, foundDefault := EpinioRoles.Default()

	user, err = authService.GetUserByUsername(ctx, email)
	if err != nil {
		if err != ErrUserNotFound {
			return user, errors.Wrap(err, "couldn't get user")
		}

		user.Username = email

		if len(userRoles) == 0 && foundDefault {
			userRoles = append(userRoles, defaultRole)
		}

		user.Roles = userRoles
		user, err = authService.SaveUser(ctx, user)
		if err != nil {
			return user, errors.Wrap(err, "couldn't create user")
		}
		return user, nil
	}

	if len(userRoles) == 0 {
		if len(user.Roles) > 0 {
			return user, nil
		}
		if foundDefault {
			userRoles = append(userRoles, defaultRole)
		}
	}

	user.Roles = userRoles
	return user, nil
}
