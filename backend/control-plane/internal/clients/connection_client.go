package clients
import("context";"errors";"github.com/jahla2/Xentra/backend/control-plane/internal/domain")
type ConnectionClient struct{runner *RunnerClient;ssh *SSHClient}
func NewConnectionClient(runner *RunnerClient,ssh *SSHClient)*ConnectionClient{return &ConnectionClient{runner:runner,ssh:ssh}}
func(c *ConnectionClient)Discover(ctx context.Context,env domain.Environment)(domain.Discovery,error){switch env.ConnectionType{case"","runner":return c.runner.Discover(ctx,env);case"ssh":return c.ssh.Discover(ctx,env);default:return domain.Discovery{},errors.New("unsupported connection type")}}
func(c *ConnectionClient)Collect(ctx context.Context,env domain.Environment)([]domain.Evidence,error){switch env.ConnectionType{case"","runner":return c.runner.Collect(ctx,env);case"ssh":return c.ssh.Collect(ctx,env);default:return nil,errors.New("unsupported connection type")}}
