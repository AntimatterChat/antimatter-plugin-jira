// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package kvstore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_hashKey(t *testing.T) {
	type args struct {
		prefix      string
		hashableKey string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"empty", args{"", ""}, ""},
		{"value", args{"", "https://mmtest.antimatter.example"}, "0886013a4dc1e7f98b3b972abdfd3f13"},
		{"prefix", args{"abc_", ""}, "abc_"},
		{"prefix value", args{"abc_", "123"}, "abc_202cb962ac59075b964b07152d234b70"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hashKey(tt.args.prefix, tt.args.hashableKey)
			require.Equal(t, tt.want, got)
		})
	}
}
