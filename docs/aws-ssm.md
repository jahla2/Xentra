# AWS Systems Manager (SSM) transport

Xentra can connect to an EC2 instance through AWS Systems Manager without opening inbound SSH.

## How it works

The control plane uses the AWS SDK default credential chain. Each Xentra environment stores only:

- AWS region
- EC2 instance ID
- optional application health URL

Xentra sends the same typed, allowlisted diagnostic and remediation operations used by the SSH and Runner transports through the AWS-managed `AWS-RunShellScript` document.

The AI does not receive a generic shell tool. It can only request Xentra tools such as:

- `system.info`
- `system.disk`
- `system.cpu`
- `system.memory`
- `system.service_status`
- `system.journal`
- `docker.list`
- `docker.logs`
- `docker.inspect`
- `docker.stats`
- `git.status`
- `git.log`
- `git.diff`
- `git.show_commit`
- `http.health_check`
- `dns.lookup`

Approved mutation tools remain limited to:

- `docker.restart`
- `system.service_restart`

## EC2 prerequisites

The target EC2 instance must:

1. be registered and online in AWS Systems Manager;
2. have SSM Agent running;
3. have an instance profile that permits Systems Manager connectivity, commonly through the AWS-managed `AmazonSSMManagedInstanceCore` policy.

No inbound TCP/22 access is required for this transport.

## Control-plane credentials

The Xentra control plane must have AWS credentials through the standard AWS SDK chain, for example:

- an EC2 instance role;
- an ECS task role;
- environment credentials;
- a mounted AWS shared credentials/config file for local development.

Do not put long-lived AWS access keys in the Xentra environment record.

## Minimum control-plane IAM permissions

Scope these permissions to the accounts, regions, instances, and SSM document used by your deployment.

Example starting point:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "ssm:SendCommand",
      "Resource": [
        "arn:aws:ssm:*::document/AWS-RunShellScript",
        "arn:aws:ec2:*:ACCOUNT_ID:instance/*"
      ]
    },
    {
      "Effect": "Allow",
      "Action": "ssm:GetCommandInvocation",
      "Resource": "*"
    }
  ]
}
```

Use tighter instance/account conditions for production.

## Xentra setup

In **Environments → Add environment**:

1. select **AWS Systems Manager (SSM)**;
2. enter the AWS region, for example `ap-southeast-2`;
3. enter the EC2 instance ID, for example `i-0123456789abcdef0`;
4. optionally configure an HTTP health endpoint;
5. select **Test & connect environment**.

Xentra immediately performs read-only discovery through SSM and records OS, hostname, CPU, memory, disk, Docker containers, and supported capabilities.

## Remediation flow

AWS SSM follows the same approval model as other Xentra transports:

1. AI investigates with read-only typed tools.
2. Xentra proposes an allowlisted action.
3. An owner rejects or approves it.
4. Xentra executes the approved action through SSM.
5. Xentra verifies process/container state and the configured HTTP health endpoint.
6. The action and result are recorded in the audit trail.
