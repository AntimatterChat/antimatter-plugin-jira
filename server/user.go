// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	jira "github.com/andygrunwald/go-jira"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/antimatterchat/antimatter-plugin-jira/server/utils/kvstore"
	"github.com/antimatterchat/antimatter-plugin-jira/server/utils/types"
)

type User struct {
	PluginVersion      string
	AntimatterUserID   types.ID   `json:"mattermost_user_id"`
	ConnectedInstances *Instances `json:"connected_instances,omitempty"`
	DefaultInstanceID  types.ID   `json:"default_instance_id,omitempty"`
}

type Connection struct {
	jira.User
	PluginVersion      string
	Oauth1AccessToken  string        `json:",omitempty"`
	Oauth1AccessSecret string        `json:",omitempty"`
	OAuth2Token        *oauth2.Token `json:",omitempty"`
	Settings           *ConnectionSettings
	SavedFieldValues   *SavedFieldValues `json:"saved_field_values,omitempty"`
	AntimatterUserID   types.ID          `json:"mattermost_user_id"`
}

type SavedFieldValues struct {
	ProjectKey string `json:"project_key,omitempty"`
	IssueType  string `json:"issue_type,omitempty"`
}

func (connection *Connection) JiraAccountID() types.ID {
	if connection.AccountID != "" {
		return types.ID(connection.AccountID)
	}

	return types.ID(connection.Name)
}

type ConnectionSettings struct {
	Notifications          bool `json:"notifications"`
	RolesForDMNotification map[string]bool
}

func (s *ConnectionSettings) String() string {
	assigneeNotifications := "Notifications for assignee: off"
	mentionNotifications := "Notifications for mention: off"
	reporterNotifications := "Notifications for reporter: off"
	watchingNotifications := "Notifications for watching: off"

	if s != nil && s.ShouldReceiveNotification(assigneeRole) {
		assigneeNotifications = "Notifications for assignee: on"
	}

	if s != nil && s.ShouldReceiveNotification(mentionRole) {
		mentionNotifications = "Notifications for mention: on"
	}

	if s != nil && s.ShouldReceiveNotification(reporterRole) {
		reporterNotifications = "Notifications for reporter: on"
	}

	if s != nil && s.ShouldReceiveNotification(watchingRole) {
		watchingNotifications = "Notifications for watching: on"
	}

	return fmt.Sprintf("\t- %s \n\t- %s \n\t- %s \n\t- %s", assigneeNotifications, mentionNotifications, reporterNotifications, watchingNotifications)
}

func NewUser(antimatterUserID types.ID) *User {
	return &User{
		AntimatterUserID:   antimatterUserID,
		ConnectedInstances: NewInstances(),
	}
}

func (p *Plugin) httpUserConnect(w http.ResponseWriter, r *http.Request, instanceID types.ID) (int, error) {
	antimatterUserID := r.Header.Get("Mattermost-User-Id")
	instance, err := p.instanceStore.LoadInstance(instanceID)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError, err)
	}

	// Users shouldn't be able to make multiple connections.
	// TODO <> this block needs to be updated. Though idk if this route will still get called?
	connection, err := p.userStore.LoadConnection(instance.GetID(), types.ID(antimatterUserID))
	if err == nil && len(connection.JiraAccountID()) != 0 {
		return respondErr(w, http.StatusBadRequest,
			errors.New("you already have a Jira account linked to your Antimatter account. Please use `/jira disconnect` to disconnect"))
	}

	redirectURL, cookie, err := instance.GetUserConnectURL(antimatterUserID)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError, err)
	}

	if cookie != nil {
		http.SetCookie(w, cookie)
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
	return http.StatusFound, nil
}

func (p *Plugin) httpUserDisconnect(w http.ResponseWriter, r *http.Request) (int, error) {
	antimatterUserID := r.Header.Get("Mattermost-User-Id")
	disconnectPayload := &struct {
		InstanceID string `json:"instance_id"`
	}{}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError,
			errors.WithMessage(err, "failed to decode request"))
	}

	err = json.Unmarshal(body, disconnectPayload)
	if err != nil {
		return respondErr(w, http.StatusInternalServerError,
			errors.WithMessage(err, "failed to unmarshal disconnect payload"))
	}

	_, err = p.DisconnectUser(disconnectPayload.InstanceID, types.ID(antimatterUserID))
	if errors.Cause(err) == kvstore.ErrNotFound {
		return respondErr(w, http.StatusNotFound,
			errors.Errorf(
				"could not complete the **disconnection** request. You do not currently have a Jira account at %q linked to your Antimatter account",
				disconnectPayload.InstanceID))
	}
	if err != nil {
		return respondErr(w, http.StatusNotFound,
			errors.Errorf("could not complete the **disconnection** request. Error: %v", err))
	}

	_, err = w.Write([]byte(`{"success": true}`))
	if err != nil {
		return http.StatusInternalServerError, errors.WithMessage(err, "failed to write response")
	}

	return http.StatusOK, nil
}

// TODO succinctly document the difference between start and connect
func (p *Plugin) httpUserStart(w http.ResponseWriter, r *http.Request, instanceID types.ID) (int, error) {
	antimatterUserID := r.Header.Get("Mattermost-User-Id")

	// If user is already connected we show them the docs
	connection, err := p.userStore.LoadConnection(instanceID, types.ID(antimatterUserID))
	if err == nil && len(connection.JiraAccountID()) != 0 {
		http.Redirect(w, r, PluginRepo, http.StatusSeeOther)
		return http.StatusSeeOther, nil
	}

	// Otherwise, attempt to connect them
	return p.httpUserConnect(w, r, instanceID)
}

func (user *User) AsConfigMap() map[string]interface{} {
	return map[string]interface{}{
		"mattermost_user_id":  user.AntimatterUserID.String(),
		"connected_instances": user.ConnectedInstances.AsConfigMap(),
		"default_instance_id": user.DefaultInstanceID.String(),
	}
}

func (p *Plugin) UpdateUserDefaults(antimatterUserID, instanceID types.ID, savedValues *SavedFieldValues) {
	user, err := p.userStore.LoadUser(antimatterUserID)
	if err != nil {
		return
	}
	if !user.ConnectedInstances.Contains(instanceID) {
		return
	}

	connection, err := p.userStore.LoadConnection(instanceID, user.AntimatterUserID)
	if err != nil {
		return
	}
	if instanceID != "" && instanceID != user.DefaultInstanceID {
		user.DefaultInstanceID = instanceID
		err = p.userStore.StoreUser(user)
		if err != nil {
			return
		}
	}

	if savedValues != nil {
		connection.SavedFieldValues = savedValues
		err = p.userStore.StoreConnection(instanceID, user.AntimatterUserID, connection)
		if err != nil {
			return
		}
	}

	info, err := p.GetUserInfo(antimatterUserID, user)
	if err != nil {
		return
	}

	p.client.Frontend.PublishWebSocketEvent(websocketEventUpdateDefaults, info.AsConfigMap(),
		&model.WebsocketBroadcast{UserId: antimatterUserID.String()},
	)
}

func (p *Plugin) httpGetSettingsInfo(w http.ResponseWriter, r *http.Request) (int, error) {
	conf := p.getConfig()
	return respondJSON(w, struct {
		UIEnabled                              bool `json:"ui_enabled"`
		SecurityLevelEmptyForJiraSubscriptions bool `json:"security_level_empty_for_jira_subscriptions"`
	}{
		UIEnabled:                              conf.EnableJiraUI,
		SecurityLevelEmptyForJiraSubscriptions: conf.SecurityLevelEmptyForJiraSubscriptions,
	})
}

func (p *Plugin) connectUser(instance Instance, antimatterUserID types.ID, connection *Connection) error {
	user, err := p.userStore.LoadUser(antimatterUserID)
	if err != nil {
		if errors.Cause(err) != kvstore.ErrNotFound {
			return err
		}
		user = NewUser(antimatterUserID)
	}
	user.ConnectedInstances.Set(instance.Common())

	err = p.userStore.StoreConnection(instance.GetID(), antimatterUserID, connection)
	if err != nil {
		return err
	}
	err = p.userStore.StoreUser(user)
	if err != nil {
		return err
	}

	_ = p.setupFlow.ForUser(string(antimatterUserID)).Go(stepConnected)

	info, err := p.GetUserInfo(antimatterUserID, user)
	if err != nil {
		return err
	}

	p.client.Frontend.PublishWebSocketEvent(websocketEventConnect, info.AsConfigMap(),
		&model.WebsocketBroadcast{UserId: antimatterUserID.String()},
	)

	p.TrackUserEvent("userConnected", antimatterUserID.String(), nil)

	return nil
}

func (p *Plugin) DisconnectUser(instanceURL string, antimatterUserID types.ID) (*Connection, error) {
	user, instance, err := p.LoadUserInstance(antimatterUserID, instanceURL)
	if err != nil {
		return nil, err
	}
	return p.disconnectUser(instance, user)
}

func (p *Plugin) SetDefaultInstance(instanceURL string, antimatterUserID types.ID) error {
	user, instance, err := p.LoadUserInstance(antimatterUserID, instanceURL)
	if err != nil {
		return err
	}

	if !user.ConnectedInstances.Contains(instance.GetID()) {
		return errors.Wrapf(kvstore.ErrNotFound, "user is not connected to %q", instance.GetID())
	}

	user.DefaultInstanceID = instance.GetID()

	if err := p.userStore.StoreUser(user); err != nil {
		return err
	}

	return nil
}

func (p *Plugin) disconnectUser(instance Instance, user *User) (*Connection, error) {
	if !user.ConnectedInstances.Contains(instance.GetID()) {
		return nil, errors.Wrapf(kvstore.ErrNotFound, "user is not connected to %q", instance.GetID())
	}
	conn, err := p.userStore.LoadConnection(instance.GetID(), user.AntimatterUserID)
	if err != nil {
		return nil, err
	}

	if user.DefaultInstanceID == instance.GetID() {
		user.DefaultInstanceID = ""
	}

	user.ConnectedInstances.Delete(instance.GetID())

	err = p.userStore.DeleteConnection(instance.GetID(), user.AntimatterUserID)
	if err != nil && errors.Cause(err) != kvstore.ErrNotFound {
		return nil, err
	}
	err = p.userStore.StoreUser(user)
	if err != nil {
		return nil, err
	}

	p.cleanupDMSubscriptionsOnDisconnect(instance.GetID(), user.AntimatterUserID.String())

	info, err := p.GetUserInfo(user.AntimatterUserID, user)
	if err != nil {
		return nil, err
	}
	p.client.Frontend.PublishWebSocketEvent(websocketEventDisconnect, info.AsConfigMap(),
		&model.WebsocketBroadcast{UserId: user.AntimatterUserID.String()})

	p.TrackUserEvent("userDisconnected", user.AntimatterUserID.String(), nil)

	return conn, nil
}

func (p *Plugin) GetJiraUserFromMentions(instanceID types.ID, mentions model.UserMentionMap, userKey string) (*jira.User, error) {
	userKey = strings.TrimPrefix(userKey, "@")
	mentionUser, found := mentions[userKey]
	if !found {
		return nil, errors.New("the mentioned user was not found")
	}

	connection, err := p.userStore.LoadConnection(instanceID, types.ID(mentionUser))
	if err != nil {
		p.client.Log.Warn("Error occurred while loading connection", "User", mentionUser, "Error", err.Error())
		return nil, errors.New("the mentioned user is not connected to Jira")
	}

	if connection.AccountID != "" {
		return &connection.User, nil
	}

	return nil, errors.New("the mentioned user is not connected to Jira")
}
