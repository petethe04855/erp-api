package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"chawy-erp-api/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.LoadConfig()
	dsn := cfg.DatabaseURL
	if dsn == "" {
		dsn = fmt.Sprintf(
			"postgres://%s:%s@%s:%s/%s?sslmode=%s",
			cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName, cfg.DBSSLMode,
		)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	defer db.Close()

	fmt.Println("=== detailed activity ===")
	rows, err := db.Query(`
		SELECT pid, state,
		       to_char(backend_start, 'HH24:MI:SS') AS backend,
		       COALESCE(to_char(xact_start, 'HH24:MI:SS'), '-') AS xact,
		       to_char(query_start, 'HH24:MI:SS') AS qstart,
		       to_char(state_change, 'HH24:MI:SS') AS schange,
		       COALESCE(backend_xid::text, '-') AS xid,
		       query
		FROM pg_stat_activity
		WHERE datname = current_database() AND pid <> pg_backend_pid()
		ORDER BY COALESCE(xact_start, backend_start)`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query:", err)
		os.Exit(1)
	}
	for rows.Next() {
		var pid int
		var state, backend, xact, qstart, schange, xid, q string
		_ = rows.Scan(&pid, &state, &backend, &xact, &qstart, &schange, &xid, &q)
		fmt.Printf("pid=%d state=%s\n  backend=%s xact=%s query_start=%s state_change=%s xid=%s\n  q=%s\n\n",
			pid, state, backend, xact, qstart, schange, xid, strings.ReplaceAll(q, "\n", " "))
	}
	rows.Close()

	fmt.Println("=== locks held by idle-in-transaction sessions ===")
	rows2, err := db.Query(`
		SELECT l.pid, a.state, l.locktype, l.mode, l.granted,
		       COALESCE(r.relname, '-') AS rel
		FROM pg_locks l
		JOIN pg_stat_activity a ON a.pid = l.pid
		LEFT JOIN pg_class r ON r.oid = l.relation
		WHERE a.state = 'idle in transaction'
		ORDER BY l.pid, l.locktype, r.relname`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query locks:", err)
		os.Exit(1)
	}
	for rows2.Next() {
		var pid int
		var state, locktype, mode, rel string
		var granted bool
		_ = rows2.Scan(&pid, &state, &locktype, &mode, &granted, &rel)
		fmt.Printf("pid=%d %-24s %-22s granted=%-5v rel=%s\n", pid, locktype, mode, granted, rel)
	}
	rows2.Close()

	fmt.Println("\n=== lock dependency (who waits on whom), detailed ===")
	rows3, err := db.Query(`
		WITH waiting AS (
			SELECT l.pid, l.locktype, l.relation, l.page, l.tuple, l.transactionid, a.query AS wq
			FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
			WHERE NOT l.granted
		)
		SELECT w.pid AS blocked, left(w.wq, 90) AS blocked_q,
		       g.pid AS blocking, a.state AS blocking_state,
		       COALESCE(r.relname, '-') AS rel, g.locktype, g.mode
		FROM waiting w
		JOIN pg_locks g ON g.locktype = w.locktype
		   AND g.relation IS NOT DISTINCT FROM w.relation
		   AND g.page IS NOT DISTINCT FROM w.page
		   AND g.tuple IS NOT DISTINCT FROM w.tuple
		   AND g.transactionid IS NOT DISTINCT FROM w.transactionid
		   AND g.granted AND g.pid <> w.pid
		JOIN pg_stat_activity a ON a.pid = g.pid
		LEFT JOIN pg_class r ON r.oid = w.relation`)
	if err != nil {
		fmt.Fprintln(os.Stderr, "query deps:", err)
		os.Exit(1)
	}
	for rows3.Next() {
		var bp, gp int
		var bq, bs, rel, locktype, mode string
		_ = rows3.Scan(&bp, &bq, &gp, &bs, &rel, &locktype, &mode)
		fmt.Printf("blocked pid=%d [%s] waits %s(%s,%s) <- blocking pid=%d state=%s\n",
			bp, bq, locktype, rel, mode, gp, bs)
	}
	rows3.Close()
}
