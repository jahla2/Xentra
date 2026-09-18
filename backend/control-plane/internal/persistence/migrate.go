package persistence
import("context";"database/sql";_ "embed")
//go:embed migrations/001_core.sql
var coreMigration string
func Migrate(ctx context.Context,db *sql.DB)error{_,err:=db.ExecContext(ctx,coreMigration);return err}
