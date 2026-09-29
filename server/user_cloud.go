// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/antimatterchat/antimatter-plugin-jira/server/utils/types"
)

const (
	argJiraJWT       = "jwt"
	argAMToken       = "mm_token"
	cookieSecretName = "jira_temp_cookie"
)

func (p *Plugin) httpACUserRedirect(w http.ResponseWriter, r *http.Request, instanceID types.ID) (int, error) {
	instance, err := p.instanceStore.LoadInstance(instanceID)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError, err)
	}
	ci, ok := instance.(*cloudInstance)
	if !ok {
		return respondErr(w, http.StatusInternalServerError,
			errors.Errorf("Not supported for instance type %s", instance.Common().Type))
	}

	_, _, err = ci.parseHTTPRequestJWT(r)
	if err != nil {
		return respondErr(w, http.StatusBadRequest, err)
	}

	submitURL := p.CreateFullURLPath(instancePath(routeACUserConfirm, instanceID))

	return ci.Plugin.respondTemplate(w, r, ContentTypeHTML, struct {
		SubmitURL  string
		ArgJiraJWT string
		ArgAMToken string
	}{
		SubmitURL:  submitURL,
		ArgJiraJWT: argJiraJWT,
		ArgAMToken: argAMToken,
	})
}

func (p *Plugin) httpACUserInteractive(w http.ResponseWriter, r *http.Request, instanceID types.ID) (int, error) {
	instance, err := p.instanceStore.LoadInstance(instanceID)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError, err)
	}
	ci, ok := instance.(*cloudInstance)
	if !ok {
		return respondErr(w, http.StatusInternalServerError,
			errors.Errorf("2 Not supported for instance type %s", instance.Common().Type))
	}

	jwtToken, _, err := ci.parseHTTPRequestJWT(r)
	if err != nil {
		return respondErr(w, http.StatusBadRequest, err)
	}
	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return respondErr(w, http.StatusBadRequest, errors.New("invalid JWT claims"))
	}
	accountID, ok := claims["sub"].(string)
	if !ok {
		return respondErr(w, http.StatusBadRequest, errors.New("invalid JWT claim sub"))
	}

	jiraClient, _, err := ci.getClientForConnection(&Connection{User: jira.User{AccountID: accountID}})
	if err != nil {
		return respondErr(w, http.StatusBadRequest, errors.Errorf("could not get client for user, err: %v", err))
	}

	jUser, _, err := jiraClient.User.GetSelf()
	if err != nil {
		return respondErr(w, http.StatusBadRequest, errors.Errorf("could not get user info for client, err: %v", err))
	}

	amToken := r.FormValue(argAMToken)
	connection := &Connection{
		PluginVersion: manifest.Version,
		User: jira.User{
			AccountID:   accountID,
			Key:         jUser.Key,
			Name:        jUser.Name,
			DisplayName: jUser.DisplayName,
		},
		// Set default settings the first time a user connects
		Settings: &ConnectionSettings{
			Notifications: true,
			RolesForDMNotification: map[string]bool{
				mentionRole:  true,
				assigneeRole: true,
				reporterRole: true,
				watchingRole: true,
			},
		},
	}

	secretCookie, err := r.Cookie(cookieSecretName)
	if err != nil {
		siteURL := p.GetSiteURL()
		return respondErr(w, http.StatusUnauthorized, errors.New(
			`Antimatter failed to recognize your user account. `+
				`Please make sure third-party cookies are enabled in your browser settings. You can disable this setting after conntecting your Jira account. `+
				`Please also make sure you are signed into Antimatter at `+siteURL))
	}

	antimatterUserID, secret, err := p.ParseAuthToken(amToken)
	if err != nil {
		return respondErr(w, http.StatusUnauthorized, err)
	}

	amuser, err := p.client.User.Get(antimatterUserID)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError,
			errors.WithMessage(err, "failed to load user "+antimatterUserID))
	}

	_, urlpath := splitInstancePath(r.URL.Path)
	switch urlpath {
	case routeACUserConnected:
		storedSecret := ""
		storedSecret, err = p.otsStore.LoadOneTimeSecret(antimatterUserID)
		if err != nil {
			return respondErr(w, http.StatusUnauthorized, err)
		}

		parsed := strings.Split(storedSecret, "-")
		if len(parsed) < 2 || parsed[0] != secret || parsed[1] != secretCookie.Value {
			return respondErr(w, http.StatusUnauthorized, errors.New("this link has already been used"))
		}
		err = p.connectUser(ci, types.ID(antimatterUserID), connection)
		if err != nil {
			return respondErr(w, http.StatusInternalServerError, err)
		}
		// TODO For https://github.com/antimatterchat/antimatter-plugin-jira/issues/149, need a channel ID
		// msg := fmt.Sprintf("You have successfully connected your Jira account (**%s**).", connection.DisplayName)
		// _ = p.client.Post.SendEphemeralPost(antimatterUserID, makePost(p.getUserID(), channelID, msg))

	case routeACUserDisconnected:
		_, err = p.DisconnectUser(ci.InstanceID.String(), types.ID(antimatterUserID))

	case routeACUserConfirm:

	default:
		return respondErr(w, http.StatusInternalServerError,
			errors.New("route "+r.URL.Path+" should be unreachable"))
	}
	if err != nil {
		return respondErr(w, http.StatusInternalServerError, err)
	}

	amDisplayName := amuser.GetDisplayName(model.ShowFullName)
	userName := amuser.GetDisplayName(model.ShowUsername)
	if amDisplayName == userName {
		amDisplayName = "@" + amDisplayName
	} else {
		amDisplayName += " (@" + userName + ")"
	}

	// This set of props should work for all relevant routes/templates
	connectSubmitURL := p.CreateFullURLPath(instancePath(routeACUserConnected, instanceID))
	disconnectSubmitURL := p.CreateFullURLPath(instancePath(routeACUserDisconnected, instanceID))
	return ci.Plugin.respondTemplate(w, r, ContentTypeHTML, struct {
		ConnectSubmitURL      string
		DisconnectSubmitURL   string
		ArgJiraJWT            string
		ArgAMToken            string
		AMToken               string
		JiraDisplayName       string
		AntimatterDisplayName string
	}{
		DisconnectSubmitURL:   disconnectSubmitURL,
		ConnectSubmitURL:      connectSubmitURL,
		ArgJiraJWT:            argJiraJWT,
		ArgAMToken:            argAMToken,
		AMToken:               amToken,
		JiraDisplayName:       jUser.DisplayName,
		AntimatterDisplayName: amDisplayName,
	})
}
