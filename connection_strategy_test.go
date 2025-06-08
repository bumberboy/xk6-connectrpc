package connectrpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectionStrategyValidation(t *testing.T) {
	tests := []struct {
		name         string
		strategy     string
		expectError  bool
		errorMessage string
	}{
		{
			name:        "Valid per-vu strategy",
			strategy:    "per-vu",
			expectError: false,
		},
		{
			name:        "Valid per-iteration strategy",
			strategy:    "per-iteration",
			expectError: false,
		},
		{
			name:        "Valid per-call strategy",
			strategy:    "per-call",
			expectError: false,
		},
		{
			name:         "Invalid strategy",
			strategy:     "invalid-strategy",
			expectError:  true,
			errorMessage: "invalid connectionStrategy: invalid-strategy. Must be 'per-vu', 'per-iteration', or 'per-call'",
		},
		{
			name:         "Empty strategy",
			strategy:     "",
			expectError:  true,
			errorMessage: "invalid connectionStrategy: . Must be 'per-vu', 'per-iteration', or 'per-call'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := newTestState(t)

			// Create client in init context
			_, err := ts.Run(`var client = new connectrpc.Client();`)
			require.NoError(t, err)

			ts.ToVUContext()

			// Test connection with the strategy
			code := `
				try {
					client.connect('test.example.com', {
						connectionStrategy: '` + tc.strategy + `',
						plaintext: true
					});
					'connected';
				} catch (e) {
					throw e;
				}
			`

			_, err = ts.Run(code)

			if tc.expectError {
				assert.Error(t, err, "Expected error for strategy %q", tc.strategy)
				assert.Contains(t, err.Error(), tc.errorMessage, "Error message mismatch")
			} else {
				assert.NoError(t, err, "Unexpected error for strategy %q", tc.strategy)
			}
		})
	}
}

func TestConnectionStrategyDefault(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Create client in init context
	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Connect without specifying connectionStrategy (should use default per-vu)
	_, err = ts.Run(`
		client.connect('test.example.com', {
			plaintext: true
		});
		'connected';
	`)
	require.NoError(t, err)
}

func TestConnectionStrategyPerCall(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Create client in init context
	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Connect with per-call strategy
	_, err = ts.Run(`
		client.connect('test.example.com', {
			connectionStrategy: 'per-call',
			plaintext: true
		});
		'connected';
	`)
	require.NoError(t, err)
}

func TestConnectionStrategyPerVuPerIteration(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Create client in init context
	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Connect with per-iteration strategy
	_, err = ts.Run(`
		client.connect('test.example.com', {
			connectionStrategy: 'per-iteration',
			plaintext: true
		});
		'connected';
	`)
	require.NoError(t, err)
}

func TestConnectionStrategyPerVu(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	// Create client in init context
	_, err := ts.Run(`var client = new connectrpc.Client();`)
	require.NoError(t, err)

	ts.ToVUContext()

	// Connect with per-vu strategy (explicit)
	_, err = ts.Run(`
		client.connect('test.example.com', {
			connectionStrategy: 'per-vu',
			plaintext: true
		});
		'connected';
	`)
	require.NoError(t, err)
}
