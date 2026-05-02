#!/bin/bash
# Install + configure Amazon CloudWatch Agent on the bi8s EC2 host.
# Sends host-level metrics (CPU, memory, disk, network) and /var/log/messages to CloudWatch.
# Required IAM: CloudWatchAgentServerPolicy (already covered by infra/tofu/modules/iam).
#
# Run on the EC2 host (Amazon Linux 2023):
#   sudo bash install-cloudwatch-agent.sh
#
# Or via SSH from local:
#   scp infra/scripts/install-cloudwatch-agent.sh ec2-user@<ip>:/tmp/
#   ssh ec2-user@<ip> "sudo bash /tmp/install-cloudwatch-agent.sh"

set -euo pipefail

PROJECT_NAME="${PROJECT_NAME:-bi8s}"
AWS_REGION="${AWS_REGION:-us-east-1}"
LOG_GROUP="/aws/ec2/${PROJECT_NAME}"

echo "[1/4] Installing amazon-cloudwatch-agent..."
if ! command -v amazon-cloudwatch-agent-ctl >/dev/null 2>&1; then
  dnf install -y amazon-cloudwatch-agent || yum install -y amazon-cloudwatch-agent
fi

echo "[2/4] Writing CloudWatch Agent config..."
mkdir -p /opt/aws/amazon-cloudwatch-agent/etc
cat > /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json <<EOF
{
  "agent": {
    "metrics_collection_interval": 60,
    "run_as_user": "root",
    "region": "${AWS_REGION}"
  },
  "metrics": {
    "namespace": "${PROJECT_NAME}/EC2",
    "append_dimensions": {
      "InstanceId": "\${aws:InstanceId}"
    },
    "metrics_collected": {
      "cpu": {
        "measurement": ["cpu_usage_idle", "cpu_usage_iowait", "cpu_usage_user", "cpu_usage_system"],
        "totalcpu": true,
        "metrics_collection_interval": 60
      },
      "mem": {
        "measurement": ["mem_used_percent", "mem_available", "mem_total"],
        "metrics_collection_interval": 60
      },
      "disk": {
        "resources": ["/"],
        "measurement": ["used_percent", "inodes_used"],
        "metrics_collection_interval": 60
      },
      "diskio": {
        "resources": ["*"],
        "measurement": ["io_time", "reads", "writes"],
        "metrics_collection_interval": 60
      },
      "net": {
        "resources": ["*"],
        "measurement": ["bytes_sent", "bytes_recv", "drop_in", "drop_out"],
        "metrics_collection_interval": 60
      }
    }
  },
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/messages",
            "log_group_name": "${LOG_GROUP}",
            "log_stream_name": "{instance_id}/messages",
            "retention_in_days": 14
          },
          {
            "file_path": "/var/log/user-data.log",
            "log_group_name": "${LOG_GROUP}",
            "log_stream_name": "{instance_id}/user-data",
            "retention_in_days": 14
          }
        ]
      }
    }
  }
}
EOF

echo "[3/4] Starting CloudWatch Agent..."
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a fetch-config \
  -m ec2 \
  -c file:/opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json \
  -s

systemctl enable amazon-cloudwatch-agent

echo "[4/4] Verifying status..."
/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl -a status

echo ""
echo "CloudWatch Agent installed and running."
echo "Namespace: ${PROJECT_NAME}/EC2"
echo "Log group: ${LOG_GROUP}"
echo "View in Grafana via the CloudWatch datasource."
