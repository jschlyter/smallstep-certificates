package provisioner

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/smallstep/assert"
	"github.com/smallstep/cli/jose"
)

func Test_newKeyStore(t *testing.T) {
	srv := generateJWKServer(2)
	defer srv.Close()
	ks, err := newKeyStore(srv.URL)
	assert.FatalError(t, err)
	defer ks.Close()

	type args struct {
		uri string
	}
	tests := []struct {
		name    string
		args    args
		want    jose.JSONWebKeySet
		wantErr bool
	}{
		{"ok", args{srv.URL}, ks.keySet, false},
		{"fail", args{srv.URL + "/error"}, jose.JSONWebKeySet{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newKeyStore(tt.args.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("newKeyStore() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(got.keySet, tt.want) {
					t.Errorf("newKeyStore() = %v, want %v", got, tt.want)
				}
				got.Close()
			}
		})
	}
}

func Test_keyStore(t *testing.T) {
	srv := generateJWKServer(2)
	defer srv.Close()

	ks, err := newKeyStore(srv.URL + "/random")
	assert.FatalError(t, err)
	defer ks.Close()
	ks.RLock()
	keySet1 := ks.keySet
	ks.RUnlock()
	// Check contents
	assert.Len(t, 2, keySet1.Keys)
	assert.Len(t, 1, ks.Get(keySet1.Keys[0].KeyID))
	assert.Len(t, 1, ks.Get(keySet1.Keys[1].KeyID))
	assert.Len(t, 0, ks.Get("foobar"))

	// Wait for rotation
	time.Sleep(5 * time.Second)

	ks.RLock()
	keySet2 := ks.keySet
	ks.RUnlock()
	if reflect.DeepEqual(keySet1, keySet2) {
		t.Error("keyStore did not rotated")
	}

	// Check contents
	assert.Len(t, 2, keySet2.Keys)
	assert.Len(t, 1, ks.Get(keySet2.Keys[0].KeyID))
	assert.Len(t, 1, ks.Get(keySet2.Keys[1].KeyID))
	assert.Len(t, 0, ks.Get("foobar"))

	// Check hits
	resp, err := srv.Client().Get(srv.URL + "/hits")
	assert.FatalError(t, err)
	hits := struct {
		Hits int `json:"hits"`
	}{}
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&hits)
	assert.FatalError(t, err)
	assert.True(t, hits.Hits > 1, fmt.Sprintf("invalid number of hits: %d is not greater than 1", hits.Hits))
}

func Test_keyStore_Get(t *testing.T) {
	srv := generateJWKServer(2)
	defer srv.Close()
	ks, err := newKeyStore(srv.URL)
	assert.FatalError(t, err)
	defer ks.Close()

	type args struct {
		kid string
	}
	tests := []struct {
		name     string
		ks       *keyStore
		args     args
		wantKeys []jose.JSONWebKey
	}{
		{"ok1", ks, args{ks.keySet.Keys[0].KeyID}, []jose.JSONWebKey{ks.keySet.Keys[0]}},
		{"ok2", ks, args{ks.keySet.Keys[1].KeyID}, []jose.JSONWebKey{ks.keySet.Keys[1]}},
		{"fail", ks, args{"fail"}, []jose.JSONWebKey(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			println(tt.name)
			if gotKeys := tt.ks.Get(tt.args.kid); !reflect.DeepEqual(gotKeys, tt.wantKeys) {
				t.Errorf("keyStore.Get() = %v, want %v", gotKeys, tt.wantKeys)
			}
		})
	}
}
