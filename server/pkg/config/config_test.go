package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_UnmarshalKeys(t *testing.T) {
	conf, err := NewConfig("", nil)
	require.NoError(t, err)

	require.NoError(t, conf.unmarshalKeys("key1: secret1"))
	require.Equal(t, "secret1", conf.Keys["key1"])
}

func TestConfig_DefaultsKept(t *testing.T) {
	const content = `room:
  empty_timeout: 10`
	conf, err := NewConfig(content, nil)
	require.NoError(t, err)
	require.Equal(t, true, conf.Room.AutoCreate)
	require.Equal(t, uint32(10), conf.Room.EmptyTimeout)
}
