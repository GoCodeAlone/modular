# Database Module Diagnostics Guide

This guide helps you troubleshoot AWS IAM authentication issues with the database module v2.

## Quick Diagnostic Checklist

When experiencing IAM authentication failures, check these in order:

- [ ] AWS credentials are available and valid
- [ ] IAM policy grants `rds-db:connect` permission
- [ ] Database user exists and has `rds_iam` role
- [ ] Region in config matches RDS instance region
- [ ] DSN format is correct
- [ ] Network connectivity to RDS is working
- [ ] Security groups allow database connections

## Enabling Diagnostic Logging

Set your application logger to debug level to see detailed IAM authentication diagnostics:

```yaml
# config.yaml
logging:
  level: debug  # or use environment variable LOG_LEVEL=debug
```

## Common Error Scenarios

### 1. "AWS IAM auth not enabled"

**Error Message:**
```
failed to create database connection: AWS IAM auth not enabled
```

**Cause:** IAM authentication is not properly configured

**Solution:**
```yaml
database:
  connections:
    writer:
      aws_iam_auth:
        enabled: true    # ← Must be set to true
        region: us-east-1
```

---

### 2. "Failed to load AWS config"

**Error Message:**
```
ERROR Failed to load AWS configuration
      error="NoCredentialProviders: no valid providers in chain"
      region=us-east-1
      possible_causes="Missing AWS credentials, invalid region, or network issues"
```

**Causes:**
- No AWS credentials available
- Invalid AWS region
- Network issues preventing access to AWS metadata service

**Diagnostic Steps:**

1. **Check AWS credentials:**
   ```bash
   aws sts get-caller-identity
   ```
   Should return your AWS account ID and ARN.

2. **Verify environment variables:**
   ```bash
   echo $AWS_ACCESS_KEY_ID
   echo $AWS_SECRET_ACCESS_KEY
   echo $AWS_REGION
   ```

3. **Check credentials file:**
   ```bash
   cat ~/.aws/credentials
   cat ~/.aws/config
   ```

4. **For EC2/ECS/EKS - verify IAM role:**
   ```bash
   # On EC2
   curl http://169.254.169.254/latest/meta-data/iam/security-credentials/

   # On ECS
   curl $AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
   ```

**Solutions:**
- **Local development:** Set AWS credentials via environment or ~/.aws/credentials
- **EC2:** Attach an IAM instance profile
- **ECS:** Use task IAM role
- **EKS:** Use IAM roles for service accounts (IRSA)

---

### 3. "Failed to extract endpoint from DSN"

**Error Message:**
```
ERROR Failed to extract RDS endpoint from DSN
      error="could not extract endpoint from DSN"
      dsn_format="Expected format: postgresql://user@host:port/db"
```

**Cause:** Invalid DSN format

**Valid DSN Formats:**
```yaml
# URL style with placeholder
dsn: "postgresql://myapp_user:$TOKEN@mydb.us-east-1.rds.amazonaws.com:5432/myappdb"

# URL style without password
dsn: "postgresql://myapp_user@mydb.us-east-1.rds.amazonaws.com:5432/myappdb"

# With options
dsn: "postgresql://myapp_user@mydb.us-east-1.rds.amazonaws.com:5432/myappdb?sslmode=require"
```

**Common Mistakes:**
- ❌ Missing `://` - `postgresql:mydb.rds.amazonaws.com`
- ❌ Missing username - `postgresql://:$TOKEN@mydb.rds.amazonaws.com`
- ❌ Missing host - `postgresql://user@/dbname`

---

### 4. "Database username not found"

**Error Message:**
```
ERROR Database username not found
      dsn_has_username=false
      config_has_db_user=false
      troubleshooting="Either include username in DSN or set aws_iam_auth.db_user in config"
```

**Cause:** No username specified in either DSN or config

**Solutions:**

**Option 1:** Include username in DSN
```yaml
# Include username after ://
dsn: "postgresql://myapp_user@mydb.rds.amazonaws.com:5432/mydb"
```

**Option 2:** Specify in config
```yaml
aws_iam_auth:
  enabled: true
  region: us-east-1
  db_user: myapp_user  # ← Specify username here
```

---

### 5. "Failed to create AWS RDS store"

**Error Message:**
```
ERROR Failed to create AWS RDS credential store
      endpoint=mydb.rds.amazonaws.com:5432
      region=us-east-1
      username=myuser
      possible_causes="Invalid AWS credentials, network issues, or incorrect endpoint"
```

**Diagnostic Steps:**

1. **Verify AWS credentials work:**
   ```bash
   aws rds describe-db-instances --region us-east-1
   ```

2. **Test IAM token generation manually:**
   ```bash
   aws rds generate-db-auth-token \
     --hostname mydb.us-east-1.rds.amazonaws.com \
     --port 5432 \
     --username myapp_user \
     --region us-east-1
   ```
   Should output a token string.

3. **Check network connectivity:**
   ```bash
   # Can you reach AWS API?
   curl -I https://rds.us-east-1.amazonaws.com

   # Can you reach RDS endpoint?
   nc -zv mydb.us-east-1.rds.amazonaws.com 5432
   ```

**Solutions:**
- Ensure AWS credentials have `rds:DescribeDBInstances` permission
- Verify network allows HTTPS to AWS APIs
- Check endpoint hostname is correct

---

### 6. "Database ping failed with IAM authentication"

**Error Message:**
```
ERROR Database ping failed with IAM authentication
      error="pq: password authentication failed for user \"myapp_user\""
      timeout=10s
      possible_causes=["IAM token generation failed", "Database user doesn't have rds_iam role", ...]
```

This is the most common production error. Follow this systematic diagnostic:

#### Step 1: Verify Database User Has IAM Role

**PostgreSQL:**
```sql
-- Connect as master user
SELECT rolname, rolcanlogin
FROM pg_roles
WHERE rolname = 'myapp_user';

-- Check if user has rds_iam role
SELECT r.rolname
FROM pg_roles r
JOIN pg_auth_members m ON r.oid = m.member
WHERE m.roleid = (SELECT oid FROM pg_roles WHERE rolname = 'rds_iam')
  AND r.rolname = 'myapp_user';
```

If the user doesn't have `rds_iam` role:
```sql
GRANT rds_iam TO myapp_user;
```

**MySQL:**
```sql
-- Check if user exists and uses IAM plugin
SELECT user, host, plugin
FROM mysql.user
WHERE user = 'myapp_user';
```

User should have `plugin = 'AWSAuthenticationPlugin'`.

If not:
```sql
CREATE USER myapp_user IDENTIFIED WITH AWSAuthenticationPlugin AS 'RDS';
GRANT ALL PRIVILEGES ON myappdb.* TO myapp_user@'%';
```

#### Step 2: Verify IAM Policy

Check your IAM policy allows `rds-db:connect`:

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

**Get your Resource ARN:**
```bash
# Get cluster resource ID
aws rds describe-db-clusters \
  --db-cluster-identifier my-cluster \
  --query 'DBClusters[0].DbClusterResourceId' \
  --output text

# Result format: cluster-ABCDEF123456
# Your ARN: arn:aws:rds-db:REGION:ACCOUNT:dbuser:cluster-ABCDEF123456/USERNAME
```

**Test IAM policy:**
```bash
aws iam simulate-principal-policy \
  --policy-source-arn arn:aws:iam::123456789012:role/my-role \
  --action-names rds-db:connect \
  --resource-arns "arn:aws:rds-db:us-east-1:123456789012:dbuser:cluster-XXXXX/myapp_user"
```

#### Step 3: Test Manual Connection

Generate a token and try connecting manually:

**PostgreSQL:**
```bash
# Generate token
TOKEN=$(aws rds generate-db-auth-token \
  --hostname mydb-instance.cluster-xyz.us-east-1.rds.amazonaws.com \
  --port 5432 \
  --username myapp_user \
  --region us-east-1)

# Try connecting
PGPASSWORD=$TOKEN psql \
  -h mydb-instance.cluster-xyz.us-east-1.rds.amazonaws.com \
  -p 5432 \
  -U myapp_user \
  -d myappdb
```

If manual connection works but application doesn't:
- Application may not have AWS credentials
- Application IAM role may lack permissions
- Network connectivity differs between your machine and application

#### Step 4: Check RDS Configuration

Verify IAM authentication is enabled on RDS:

```bash
aws rds describe-db-instances \
  --db-instance-identifier my-instance \
  --query 'DBInstances[0].IAMDatabaseAuthenticationEnabled'

# Should return: true
```

If false, enable it:
```bash
aws rds modify-db-instance \
  --db-instance-identifier my-instance \
  --enable-iam-database-authentication \
  --apply-immediately
```

#### Step 5: Check Security Groups

Verify security groups allow connections from your application:

```bash
# Get security group
aws rds describe-db-instances \
  --db-instance-identifier my-instance \
  --query 'DBInstances[0].VpcSecurityGroups'

# Check security group rules
aws ec2 describe-security-groups \
  --group-ids sg-xxxxx \
  --query 'SecurityGroups[0].IpPermissions'
```

Ensure:
- Port 5432 (PostgreSQL) or 3306 (MySQL) is open
- Source includes your application's IP/security group
- VPC routing allows traffic

#### Step 6: Check CloudTrail Logs

Review IAM authentication attempts in CloudTrail:

```bash
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=EventName,AttributeValue=connect \
  --max-results 50 \
  --query 'Events[?contains(CloudTrailEvent, `rds-db:connect`)]'
```

Look for:
- Denied events (IAM policy issue)
- No events (credentials not being used)
- Errors in the event details

---

### 7. SSL/TLS Configuration Issues

**Error Message:**
```
ERROR Database ping failed
      error="pq: SSL is not enabled on the server"
```

**Solution:**

Ensure your DSN includes SSL mode:
```yaml
dsn: "postgresql://user@host:5432/db?sslmode=require"
```

For RDS, always use `sslmode=require` or `sslmode=verify-full`.

---

## Getting Help

If you've followed all diagnostic steps and still have issues:

1. **Enable debug logging** and collect logs
2. **Test manual connection** with generated token
3. **Check CloudTrail** for IAM authentication attempts
4. **Verify RDS metrics** in CloudWatch
5. **Check application logs** for any IAM-related warnings

Include this information when seeking help:
- Debug log output from application startup
- Result of manual token generation and connection
- IAM policy document
- Database user permissions (`\du` in PostgreSQL)
- CloudTrail events for connection attempts

## Diagnostic Commands Summary

```bash
# AWS Credentials
aws sts get-caller-identity
aws configure list

# IAM Token Generation
aws rds generate-db-auth-token \
  --hostname HOST \
  --port 5432 \
  --username USER \
  --region REGION

# Database User Check (PostgreSQL)
psql -h HOST -U master_user -d postgres -c "\du+ myapp_user"

# RDS IAM Status
aws rds describe-db-instances \
  --db-instance-identifier INSTANCE \
  --query 'DBInstances[0].IAMDatabaseAuthenticationEnabled'

# Security Group Check
aws ec2 describe-security-groups --group-ids SG_ID

# CloudTrail Events
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=EventName,AttributeValue=connect \
  --max-results 10

# Manual Connection Test
PGPASSWORD=$(aws rds generate-db-auth-token \
  --hostname HOST --port 5432 --username USER --region REGION) \
psql -h HOST -p 5432 -U USER -d DATABASE
```

## See Also

- [AWS_IAM_AUTH.md](AWS_IAM_AUTH.md) - Complete IAM authentication guide
- [Configuration Example](examples/aws-iam-auth-config.yaml) - YAML configuration example
- [AWS RDS IAM Documentation](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html)
