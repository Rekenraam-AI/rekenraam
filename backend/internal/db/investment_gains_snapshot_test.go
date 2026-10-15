package db_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"rekenraam/backend/internal/db"
	"rekenraam/backend/internal/testdb"
)

func TestLiveGainsSnapshotIsPrivateDisposableAndIndependent(t *testing.T) {
	t.Parallel()
	source, _ := testdb.Open(t)
	_, err := source.Exec(`CREATE TABLE snapshot_probe(value TEXT); INSERT INTO snapshot_probe VALUES ('original')`)
	require.NoError(t, err)
	snapshot, cleanup, err := db.NewInvestmentRepository(source).LiveGainsSnapshot(context.Background())
	require.NoError(t, err)
	defer cleanup()
	var seq int
	var name, path string
	require.NoError(t, snapshot.QueryRow(`PRAGMA database_list`).Scan(&seq, &name, &path))
	for target, mode := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(target)
		require.NoError(t, err)
		assert.Equal(t, mode, info.Mode().Perm())
	}
	_, err = source.Exec(`UPDATE snapshot_probe SET value = 'corrected'`)
	require.NoError(t, err)
	var value string
	require.NoError(t, snapshot.QueryRow(`SELECT value FROM snapshot_probe`).Scan(&value))
	assert.Equal(t, "original", value)
	_, err = snapshot.Exec(`UPDATE snapshot_probe SET value = 'temporary replay'`)
	require.NoError(t, err)
	require.NoError(t, source.QueryRow(`SELECT value FROM snapshot_probe`).Scan(&value))
	assert.Equal(t, "corrected", value)
	cleanup()
	assert.Error(t, snapshot.Ping(), "the private pool is closed")
	_, err = os.Stat(filepath.Dir(path))
	assert.True(t, os.IsNotExist(err), "all image and sidecar files are removed")
}

// The padding is synthetic: this measures capture/open/cleanup cost by image
// size, not a claim about end-to-end replay latency on a representative book.
func BenchmarkLiveGainsSnapshot(b *testing.B) {
	for _, mib := range []int{0, 16, 64} {
		b.Run(fmt.Sprintf("padding_%dMiB", mib), func(b *testing.B) {
			source, _ := testdb.Open(b)
			if mib > 0 {
				_, err := source.Exec(`CREATE TABLE snapshot_padding(value BLOB); INSERT INTO snapshot_padding VALUES (zeroblob(?))`, mib*1024*1024)
				require.NoError(b, err)
			}
			var pages, pageSize int64
			require.NoError(b, source.QueryRow(`PRAGMA page_count`).Scan(&pages))
			require.NoError(b, source.QueryRow(`PRAGMA page_size`).Scan(&pageSize))
			b.SetBytes(pages * pageSize)
			repository := db.NewInvestmentRepository(source)
			b.ResetTimer()
			for b.Loop() {
				_, cleanup, err := repository.LiveGainsSnapshot(context.Background())
				require.NoError(b, err)
				cleanup()
			}
			b.ReportMetric(float64(pages*pageSize), "snapshot_bytes")
		})
	}
}
