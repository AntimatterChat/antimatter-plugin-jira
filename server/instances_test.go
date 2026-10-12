// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"path/filepath"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstallInstance(t *testing.T) {
	p := &Plugin{}

	for name, tc := range map[string]struct {
		numInstances int
	}{
		"0 preinstalled": {
			numInstances: 0,
		},
		"1 preinstalled": {
			numInstances: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			api := &plugintest.API{}

			p.SetAPI(api)
			p.client = pluginapi.NewClient(api, p.Driver)
			p.instanceStore = p.getMockInstanceStoreKV(tc.numInstances)

			api.On("KVGet", mock.Anything).Return(mock.Anything, nil)
			api.On("GetConfig").Return(&model.Config{})
			api.On("UnregisterCommand", mock.Anything, mock.Anything).Return(nil)
			api.On("RegisterCommand", mock.Anything, mock.Anything).Return(nil)
			api.On("PublishWebSocketEvent", mock.Anything, mock.Anything, mock.Anything)

			path, err := filepath.Abs("..")
			require.Nil(t, err)
			api.On("GetBundlePath").Return(path, nil)

			testInstance0 := &testInstance{
				InstanceCommon: InstanceCommon{
					InstanceID: mockInstance3URL,
					IsV2Legacy: true,
					Type:       "testInstanceType",
				},
			}

			err = p.InstallInstance(testInstance0)
			assert.Nil(t, err)
		})
	}
}
