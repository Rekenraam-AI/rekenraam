package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// frozenMigrationChecksums pins the current v0.1 candidate baseline. ADR 0013
// permits an explicitly declared pre-release redesign before installations;
// installed-release migrations are immutable.
var frozenMigrationChecksums = map[string]string{
	"0001_initial_schema.sql": "a016ba8e83ad7e5f4447972d3cad2a54dfaf50e7416aad9f13cc6840c08ef66a",
}

func TestPinnedMigrationChecksums(t *testing.T) {
	for name, want := range frozenMigrationChecksums {
		contents, err := FS.ReadFile(name)
		require.NoErrorf(t, err, "pinned migration %s must not be renamed or removed", name)
		sum := sha256.Sum256(contents)
		assert.Equalf(t, want, hex.EncodeToString(sum[:]),
			"pinned migration %s changed; restore it or document an ADR-approved pre-release redesign and update this checksum", name)
	}
}
