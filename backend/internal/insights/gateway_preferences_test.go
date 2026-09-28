package insights

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayPreferenceProviderPostgres(t *testing.T) {
	dsn := os.Getenv("INSIGHTS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("INSIGHTS_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	// CTE fixtures exercise the actual query expression without creating tables.
	fixtures := `WITH accounts(id,platform) AS (VALUES(1,'openai'),(2,'antigravity')),
 usage_logs(request_id,user_id,api_key_id,account_id,requested_model,model) AS (VALUES
 ('good',1,1,1,'alias','upstream'),('wrong-user',2,1,1,'alias','upstream'),
 ('wrong-key',1,2,1,'alias','upstream'),('wrong-model',1,1,1,'other','upstream'),
 ('duplicate',1,1,1,'alias','upstream'),('duplicate',1,1,2,'alias','upstream'),
 ('missing-model',1,1,2,'gemini','upstream')),
 insights_call_facts(request_id,user_id,api_key_id,platform,model) AS (VALUES
 ('good',1,1,'composite','alias'),('good',1,1,'openai','alias'),
 ('wrong-user',1,1,'composite','alias'),('wrong-key',1,1,'composite','alias'),
 ('wrong-model',1,1,'composite','alias'),('duplicate',1,1,'composite','alias'),
 ('missing-model',1,1,'',''),('absent',1,1,'composite','alias')) `
	rows, err := db.QueryContext(context.Background(), fixtures+`SELECT `+preferenceCallModelSQL()+`,COUNT(*) FROM `+preferenceCallSourceSQL+` GROUP BY 1`)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	got := map[string]int64{}
	for rows.Next() {
		var name string
		var n int64
		require.NoError(t, rows.Scan(&name, &n))
		got[name] = n
	}
	require.NoError(t, rows.Err())
	require.Equal(t, map[string]int64{"openai:alias": 2, "unknown:alias": 5, "antigravity:gemini": 1}, got)
	var archived string
	require.NoError(t, db.QueryRow(fmt.Sprintf(`SELECT %s||':alias' FROM (VALUES('composite')) d(platform)`, usageProviderSQL("d.platform", "NULL"))).Scan(&archived))
	require.Equal(t, "unknown:alias", archived)
}
