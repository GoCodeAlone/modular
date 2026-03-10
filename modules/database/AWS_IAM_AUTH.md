# AWS IAM Authentication for RDS

## Overview

The database module v2 supports AWS RDS IAM authentication with automatic token generation and refresh. This eliminates the need to manage database passwords and provides enhanced security through AWS IAM.

## Key Features

- **Automatic Token Generation**: IAM auth tokens are automatically generated using AWS credentials
- **Automatic Token Refresh**: Tokens are refreshed before expiration (15-minute lifetime)
- **Password Placeholder Handling**: Any password in the DSN (including placeholders like `$TOKEN`) is automatically stripped and replaced
- **Connection Recovery**: Connections are automatically recreated on authentication failures
- **Zero Configuration Password Management**: No need to manually manage or rotate database passwords

## How It Works

### 1. Password Stripping

When AWS IAM authentication is enabled, **any password in the DSN is ignored and stripped**. This means you can use placeholder values for backward compatibility or clarity:

```yaml
# All of these DSN formats work identically with IAM auth:
dsn: "postgresql://myapp_user:$TOKEN@host.rds.amazonaws.com:5432/mydb"
dsn: "postgresql://myapp_user:PLACEHOLDER@host.rds.amazonaws.com:5432/mydb"
dsn: "postgresql://myapp_user@host.rds.amazonaws.com:5432/mydb"
```

The password portion (`$TOKEN`, `PLACEHOLDER`, or empty) is completely ignored when IAM auth is enabled.

### 2. Username Extraction

The database username is extracted from the DSN or can be explicitly specified:

```yaml
# Option 1: Username in DSN (extracted automatically)
dsn: "postgresql://myapp_user:$TOKEN@host.rds.amazonaws.com:5432/mydb"
aws_iam_auth:
  enabled: true
  region: us-east-1

# Option 2: Username in config (overrides DSN username)
dsn: "postgresql://ignored_user:$TOKEN@host.rds.amazonaws.com:5432/mydb"
aws_iam_auth:
  enabled: true
  region: us-east-1
  db_user: myapp_user  # This takes precedence
```

### 3. Token Generation Flow

1. Module strips any password from the DSN
2. Username is extracted from DSN or config
3. AWS credentials are loaded (environment, instance profile, etc.)
4. RDS IAM auth token is generated using AWS SDK
5. Connection is established using the generated token
6. Token is automatically refreshed before expiration

## Configuration Example

### YAML Configuration

```yaml
database:
  default: writer
  connections:
    writer:
      driver: postgres
      # DSN with $TOKEN placeholder - will be automatically stripped
      dsn: "postgresql://myapp_user:$TOKEN@mydb-instance.cluster-xyz.us-east-1.rds.amazonaws.com:5432/myappdb?sslmode=require"
      max_open_connections: 25
      max_idle_connections: 10
      connection_max_lifetime: 1h
      connection_max_idle_time: 30m
      aws_iam_auth:
        enabled: true
        region: us-east-1
        # db_user is optional - extracted from DSN if not specified
        connection_timeout: 10s
```

### Environment Variable Configuration

```bash
export DB_WRITER_DRIVER=postgres
export DB_WRITER_DSN="postgresql://myapp_user:$TOKEN@host.rds.amazonaws.com:5432/mydb?sslmode=require"
export DB_WRITER_AWS_IAM_AUTH_ENABLED=true
export DB_WRITER_AWS_IAM_AUTH_REGION=us-east-1
export DB_WRITER_MAX_OPEN_CONNECTIONS=25
```

## Prerequisites

### 1. RDS Configuration

Enable IAM authentication on your RDS instance:
- For PostgreSQL: Set `rds.force_ssl=1` and enable IAM authentication
- For MySQL: Enable IAM authentication in the RDS console

### 2. Database User Setup

Create a database user configured for IAM authentication:

**PostgreSQL:**
```sql
CREATE USER myapp_user WITH LOGIN;
GRANT rds_iam TO myapp_user;
GRANT ALL PRIVILEGES ON DATABASE myappdb TO myapp_user;
```

**MySQL:**
```sql
CREATE USER myapp_user IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';
GRANT ALL PRIVILEGES ON myappdb.* TO myapp_user@'%';
```

### 3. IAM Policy

The AWS principal (user/role) must have `rds-db:connect` permission:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["rds-db:connect"],
      "Resource": [
        "arn:aws:rds-db:us-east-1:123456789012:dbuser:cluster-XXXXX/myapp_user"
      ]
    }
  ]
}
```

**Finding your Resource ARN:**
- Format: `arn:aws:rds-db:REGION:ACCOUNT:dbuser:RESOURCE_ID/DB_USERNAME`
- Get RESOURCE_ID from RDS console (cluster identifier starts with `cluster-`)
- Example: `arn:aws:rds-db:us-east-1:123456789012:dbuser:cluster-ABC123DEF456/myapp_user`

### 4. AWS Credentials

The module uses the standard AWS SDK credential chain:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM instance profile (when running on EC2)
4. IAM role (when running on ECS/EKS)

## Migration from Password-Based Authentication

If you're currently using password-based authentication and want to migrate to IAM:

### Before (with password):
```yaml
database:
  connections:
    writer:
      driver: postgres
      dsn: "postgresql://myuser:MySecretP@ssword@host.rds.amazonaws.com:5432/mydb"
```

### After (with IAM):
```yaml
database:
  connections:
    writer:
      driver: postgres
      # Replace password with $TOKEN placeholder (or remove it entirely)
      dsn: "postgresql://myuser:$TOKEN@host.rds.amazonaws.com:5432/mydb"
      aws_iam_auth:
        enabled: true
        region: us-east-1
```

**The password portion is completely ignored when IAM auth is enabled.**

## Example Use Case

Here is a complete example DSN for an RDS Aurora PostgreSQL cluster:
```
postgresql://myapp_user:$TOKEN@mydb-instance.cluster-abc123def456.us-east-1.rds.amazonaws.com:5432/myappdb?sslmode=require
```

**This is the correct format.** Here's what happens:

1. ✅ The module sees `aws_iam_auth.enabled: true`
2. ✅ The `$TOKEN` placeholder is automatically stripped from the DSN
3. ✅ The username `myapp_user` is extracted and used for IAM authentication
4. ✅ AWS credentials are loaded from your environment
5. ✅ An RDS IAM token is generated automatically
6. ✅ The token is refreshed every ~15 minutes automatically
7. ✅ No manual password management required!

## Diagnostics and Troubleshooting

### Diagnostic Logging

The database module v2 includes comprehensive diagnostic logging for IAM authentication. When IAM auth fails, you'll see detailed error messages with troubleshooting steps.

**Enable Debug Logging** to see the complete IAM authentication flow:

```go
// Set your logger to debug level to see detailed diagnostics
app.Logger().SetLevel("debug")
```

**What Gets Logged:**

1. **Configuration Phase:**
   - AWS region being used
   - Database driver type
   - DSN processing (without exposing sensitive data)
   - Username extraction

2. **Setup Phase:**
   - AWS configuration loading
   - RDS endpoint extraction
   - Database name and options parsing
   - Credential store creation

3. **Connection Phase:**
   - Database connector creation
   - Connection pool configuration
   - Connection test (ping) results

4. **Error Details:**
   - Specific error messages with context
   - Possible causes for each failure
   - Actionable troubleshooting steps

**Example Log Output (Debug Level):**

```
INFO  Starting AWS IAM authentication setup region=us-east-1 driver=postgres
DEBUG Loading AWS configuration region=us-east-1
DEBUG AWS configuration loaded successfully
DEBUG Processing DSN for IAM authentication original_dsn_length=142
DEBUG Password stripped from DSN cleaned_dsn_length=128
INFO  Extracted RDS endpoint endpoint=mydb.cluster-xyz.us-east-1.rds.amazonaws.com:5432
DEBUG Extracted database configuration database=mydb options_count=1
DEBUG Extracted username from DSN username=myapp_user
INFO  IAM authentication will use database user username=myapp_user
DEBUG Determined database driver configuration driver=pgx port=5432
INFO  Creating AWS RDS credential store endpoint=mydb... region=us-east-1 username=myapp_user
DEBUG AWS RDS credential store created successfully
INFO  Database connection with AWS IAM authentication configured successfully
DEBUG Testing database connection timeout=10s
INFO  Database connection test successful
```

**Example Error Output:**

```
ERROR Failed to load AWS configuration
      error="NoCredentialProviders: no valid providers in chain"
      region=us-east-1
      possible_causes="Missing AWS credentials, invalid region, or network issues"
```

### Connection Failures

When experiencing connection failures, the module provides detailed diagnostics:

**1. AWS Configuration Errors**

If you see:
```
ERROR Failed to load AWS configuration
      error="NoCredentialProviders: no valid providers in chain"
```

**Solutions:**
- Set AWS environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`
- Configure `~/.aws/credentials` file
- Ensure IAM instance profile is attached (EC2/ECS/EKS)
- Verify AWS region is valid

**2. Endpoint Extraction Errors**

If you see:
```
ERROR Failed to extract RDS endpoint from DSN
      error="could not extract endpoint from DSN"
      dsn_format="Expected format: postgresql://user@host:port/db"
```

**Solutions:**
- Verify DSN format is correct
- Example: `postgresql://user@host.rds.amazonaws.com:5432/dbname`
- Ensure host:port is properly formatted

**3. Username Not Found**

If you see:
```
ERROR Database username not found
      dsn_has_username=false
      config_has_db_user=false
```

**Solutions:**
- Include username in DSN: `postgresql://username@host/db`
- OR set `aws_iam_auth.db_user` in configuration
- Verify the username exists in the database

**4. Credential Store Creation Failed**

If you see:
```
ERROR Failed to create AWS RDS credential store
      endpoint=mydb.rds.amazonaws.com:5432
      region=us-east-1
      username=myuser
      possible_causes="Invalid AWS credentials, network issues, or incorrect endpoint"
```

**Solutions:**
- Verify AWS credentials have necessary permissions
- Test AWS connectivity: `aws sts get-caller-identity`
- Check network connectivity to AWS APIs
- Verify endpoint is correct RDS hostname

**5. Connection Ping Failures**

If you see:
```
ERROR Database ping failed with IAM authentication
      error="pq: password authentication failed"
      timeout=10s
      possible_causes=["IAM token generation failed", "Database user doesn't have rds_iam role", ...]
```

**Solutions - Check each possible cause:**

1. **Verify IAM policy**: Ensure `rds-db:connect` permission is granted
2. **Check region**: Ensure `aws_iam_auth.region` matches your RDS region
3. **Verify username**: Ensure the database user exists and has `rds_iam` role (PostgreSQL)
4. **AWS credentials**: Verify AWS credentials are available (`aws sts get-caller-identity`)
5. **Network connectivity**: Ensure security groups allow connections from your application
6. **Database user setup**: Check the user has the `rds_iam` role granted

### Token Expiration

Tokens are automatically refreshed by the `go-db-credential-refresh` library. If you see authentication errors:

- The library automatically retries with a fresh token
- Check logs for "credential refresh" messages
- Ensure your application has continuous AWS credentials access

### Testing IAM Authentication

You can test IAM authentication manually:

```bash
# Generate a token
TOKEN=$(aws rds generate-db-auth-token \
  --hostname mydb-instance.cluster-xyz.us-east-1.rds.amazonaws.com \
  --port 5432 \
  --username myapp_user \
  --region us-east-1)

# Connect using the token
PGPASSWORD=$TOKEN psql \
  -h mydb-instance.cluster-xyz.us-east-1.rds.amazonaws.com \
  -p 5432 \
  -U myapp_user \
  -d myappdb
```

## Benefits of IAM Authentication

1. **No Password Management**: No need to store or rotate database passwords
2. **Centralized Access Control**: Use IAM policies to control database access
3. **Audit Trail**: All authentication attempts are logged in CloudTrail
4. **Short-Lived Credentials**: Tokens expire after 15 minutes
5. **Automatic Rotation**: Tokens are automatically refreshed
6. **AWS Integration**: Works seamlessly with other AWS services

## See Also

- [Configuration Example](examples/aws-iam-auth-config.yaml)
- [AWS RDS IAM Authentication Documentation](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html)
- [go-db-credential-refresh Library](https://github.com/davepgreene/go-db-credential-refresh)
