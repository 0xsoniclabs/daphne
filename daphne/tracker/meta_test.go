package tracker

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadata_Keys_ProducesOrderedListOfKeys(t *testing.T) {
	require := require.New(t)
	meta := Metadata{
		data: map[string]any{
			"b": "2",
			"a": "1",
			"c": "3",
		},
	}

	keys := meta.Keys()
	require.Equal(keys, []string{"a", "b", "c"})
}

func TestMetadata_Get_ReturnsValueForKey(t *testing.T) {
	require := require.New(t)
	meta := Metadata{
		data: map[string]any{
			"key1": "value1",
			"key2": "value2",
		},
	}

	require.Equal("value1", meta.Get("key1"))
	require.Equal("value2", meta.Get("key2"))
	require.Equal("", meta.Get("key3"))
}

func TestMetadata_String_ProducesOrderedListOfKeys(t *testing.T) {
	require := require.New(t)
	meta := Metadata{
		data: map[string]any{
			"b": "2",
			"a": "1",
			"c": "3",
		},
	}

	result := meta.String()
	expected := "{a: 1, b: 2, c: 3}"
	require.Equal(expected, result)
}
