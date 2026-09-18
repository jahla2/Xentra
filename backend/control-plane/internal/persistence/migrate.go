package persistence
import("context";"database/sql";_ "embed")
//go:embed migrations/001_core.sql
var coreMigration string
//go:embed migrations/002_incidents.sql
var incidentMigration string
func Migrate(ctx context.Context,db *sql.DB)error{for _,migration:=range []string{coreMigration,incidentMigration}{if _,err:=db.ExecContext(ctx,migration);err!=nil{return err}};return nil}
