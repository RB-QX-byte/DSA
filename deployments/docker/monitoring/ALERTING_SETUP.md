# Judge System Alerting Setup

## Overview

This document describes how to configure comprehensive alerting for the Judge System using Prometheus Alertmanager with multiple notification channels.

## Alert Rules Configuration

The system includes the following alert rules (`alert_rules.yml`):

### API Performance Alerts
- **HighAPILatency**: Triggers when 95th percentile latency > 100ms (warning)
- **CriticalAPILatency**: Triggers when 95th percentile latency > 500ms (critical)
- **HighErrorRate**: Triggers when error rate > 5% (critical)

### Judge System Alerts
- **JudgeQueueBackup**: Triggers when default queue > 50 pending submissions (warning)
- **CriticalJudgeQueueBackup**: Triggers when default queue > 200 pending submissions (critical)
- **CriticalQueueBackup**: Triggers when critical queue > 10 pending submissions (critical)

### System Resource Alerts
- **HighMemoryUsage**: Triggers when memory usage > 90% (warning)
- **HighCPUUsage**: Triggers when CPU usage > 80% (warning)

### Infrastructure Alerts
- **DatabaseConnectionFailure**: Triggers on database connection issues (critical)
- **RedisConnectionFailure**: Triggers on Redis connection issues (critical)
- **ServiceDown**: Triggers when any service is down (critical)

## Notification Channels

### 1. Slack Integration

To set up Slack notifications:

1. Create a Slack app and get webhook URLs
2. Replace `YOUR_SLACK_API_URL_HERE` in `alertmanager.yml` with your actual webhook URL
3. Configure the following channels:
   - `#alerts-critical`: For critical alerts
   - `#alerts-warning`: For warning alerts
   - `#judge-system`: For judge-specific alerts

### 2. Webhook Integration

The system also supports generic webhook notifications to `http://localhost:5001/webhook` with the following endpoints:
- `/webhook` - Default notifications
- `/webhook/critical` - Critical alerts
- `/webhook/warning` - Warning alerts
- `/webhook/judge` - Judge-specific alerts

### 3. Email Notifications (Optional)

Configure SMTP settings in the `global` section of `alertmanager.yml`:
```yaml
global:
  smtp_smarthost: 'your-smtp-server:587'
  smtp_from: 'alerts@your-domain.com'
```

## Alert Routing

The system uses intelligent routing based on severity and service:

1. **Critical alerts** (severity: critical):
   - Immediate notification (5s group wait)
   - Repeat every 15 minutes
   - Sent to critical channels

2. **Warning alerts** (severity: warning):
   - Delayed notification (30s group wait)
   - Repeat every hour
   - Sent to warning channels

3. **Judge-specific alerts** (service: judge):
   - Custom routing (15s group wait)
   - Repeat every 30 minutes
   - Sent to judge system channels

## Grafana Dashboard Integration

The alerts are designed to work with the existing Grafana dashboards:

- **System Overview Dashboard** (`judge-system-overview`): Shows all key metrics
- **Distributed Tracing Dashboard** (`judge-system-tracing`): Shows trace performance

## Testing Alerts

To test the alerting system:

1. **Test API latency alerts**:
   ```bash
   # Generate load to increase latency
   ab -n 1000 -c 50 http://localhost:8080/api/submissions
   ```

2. **Test queue backup alerts**:
   ```bash
   # Submit many tasks to fill the queue
   for i in {1..100}; do
     curl -X POST http://localhost:8080/api/submissions -d '{"code":"print(1)"}'
   done
   ```

3. **Test service down alerts**:
   ```bash
   # Stop a service temporarily
   docker-compose stop api-server
   # Wait 1 minute for alert to trigger
   docker-compose start api-server
   ```

## Customization

### Adding New Alert Rules

1. Edit `alert_rules.yml`
2. Add your rule under the `judge_system_alerts` group
3. Follow the existing pattern for labels and annotations
4. Restart Prometheus to reload rules

### Adding New Notification Channels

1. Edit `alertmanager.yml`
2. Add new receiver configuration
3. Update routing rules as needed
4. Restart Alertmanager

### PagerDuty Integration

To add PagerDuty integration, add to a receiver:

```yaml
pagerduty_configs:
  - service_key: 'YOUR_PAGERDUTY_SERVICE_KEY'
    description: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'
    severity: '{{ .CommonLabels.severity }}'
```

## Monitoring the Alerting System

- **Prometheus Targets**: http://localhost:9090/targets
- **Alertmanager Status**: http://localhost:9093
- **Active Alerts**: http://localhost:9093/#/alerts
- **Silence Management**: http://localhost:9093/#/silences

## Security Considerations

1. Store Slack webhook URLs and API keys securely
2. Use environment variables for sensitive configuration
3. Implement proper authentication for webhook endpoints
4. Consider using TLS for webhook communications

## Troubleshooting

### Common Issues

1. **Alerts not firing**: Check Prometheus rule evaluation
2. **Notifications not sent**: Verify Alertmanager configuration
3. **Slack messages not received**: Check webhook URL and channel permissions
4. **Wrong alert thresholds**: Adjust expressions in `alert_rules.yml`

### Debugging Commands

```bash
# Check alert rule syntax
promtool check rules alert_rules.yml

# Check alertmanager config
amtool check-config alertmanager.yml

# View active alerts
curl http://localhost:9090/api/v1/alerts

# Test alertmanager routing
amtool config routes test --config.file=alertmanager.yml
```