package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		expectedErr bool
	}{
		{
			name:        "Invalid Password length",
			password:    "Halkjiokajsdklqmklwjemkoqjwdkasjmkldmaklsjmdlkqjwmekljqlkwjdmklajmdslkajskldjaklsdjqkljwdkljmsklajdklasjdlkjaklsjdlkajsdklajsdkljaskldjq",
			expectedErr: true,
		},
		{
			name:        "Valid Password length",
			password:    "vikjsqwenaklmsiodjqw",
			expectedErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nPass := Password{}
			err := nPass.Set(tc.password)
			if tc.expectedErr {
				assert.Error(t, err, "expected error got none")
			} else {
				assert.NoError(t, err, "expected error to be nil but got one")
				assert.LessOrEqual(t, len(*nPass.plaintext), 72, "expected the length be lesser than 72")
				assert.NotEqual(t, len(nPass.hash), 0, "expected caculated hash but got nothing")
			}
		})
	}

}

func TestMatch(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		hashValue   string
		expectedErr bool
	}{
		{
			name:        "Valid password hash",
			password:    "lkaskdjqoiwjeioqjwoie",
			hashValue:   "$2a$12$faQ1M6zprk9x8afrofQBr.1GKxDSdKUFDUNdOxmVegPhzTxt/qsmC",
			expectedErr: false,
		},
		{
			name:        "Invalid password hash",
			password:    "lkaskdjqoiwjeioqjwoie",
			hashValue:   "$2a$12$faQ1M6zprk9x8afrofQBr.wrongvalueDUNdOxmVegPhzTxt/qsmC",
			expectedErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nPass := Password{}
			nPass.plaintext = &tc.password
			nPass.hash = []byte(tc.hashValue)
			ok, err := nPass.Match()
			if tc.expectedErr {
				assert.False(t, ok, "expected the hash value to be wrong")
			} else {
				assert.NoError(t, err, "expected error to be nil but got one")
				assert.True(t, ok, "expected that the password and hash match together but mismatch happened")
			}
		})
	}
}
