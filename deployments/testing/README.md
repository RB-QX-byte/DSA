# Judge System Security and Load Testing Suite

This directory contains comprehensive testing configurations for validating the security, performance, and resilience of the enhanced judge system.

## Testing Components

### 1. Penetration Testing (`penetration-testing.yaml`)

**Purpose**: Validate security controls and identify vulnerabilities

**Features**:
- Network scanning and vulnerability assessment
- Container escape attempt testing
- Resource exhaustion protection validation
- Security control verification

**Tools Used**:
- Nmap for network scanning
- Custom scripts for container security testing
- Kali Linux based testing environment

**Usage**:
```bash
kubectl apply -f penetration-testing.yaml
kubectl logs -n security-testing job/penetration-test-runner -f
```

### 2. Load Testing (`load-testing.yaml`)

**Purpose**: Validate system performance under various load conditions

**Features**:
- Concurrent submission testing
- Burst traffic simulation
- Malicious code handling validation
- Performance threshold verification

**Scenarios**:
- **Load Test**: Gradual ramp-up to sustained load
- **Stress Test**: High concurrent submissions with security validation
- **Scheduled Test**: Daily automated performance validation

**Tools Used**:
- K6 for load generation
- Custom JavaScript test scenarios
- Prometheus integration for metrics

**Usage**:
```bash
kubectl apply -f load-testing.yaml
kubectl logs -n security-testing job/load-test-runner -f
kubectl logs -n security-testing job/stress-test-runner -f
```

### 3. Chaos Engineering (`chaos-engineering.yaml`)

**Purpose**: Validate system resilience and fault tolerance

**Features**:
- Pod failure simulation
- Network disruption testing
- Resource exhaustion scenarios
- Recovery validation

**Chaos Tests**:
- **Pod Killer**: Random pod termination and recovery testing
- **Network Chaos**: Latency and packet loss simulation
- **Resource Exhaustion**: CPU, memory, and I/O stress testing

**Usage**:
```bash
kubectl apply -f chaos-engineering.yaml
kubectl logs -n chaos-engineering job/chaos-pod-killer -f
kubectl logs -n chaos-engineering job/chaos-network-test -f
kubectl logs -n chaos-engineering job/chaos-resource-exhaustion -f
```

### 4. Security Compliance Scanning (`security-compliance-scan.yaml`)

**Purpose**: Validate compliance with security standards and best practices

**Features**:
- CIS Kubernetes Benchmark compliance
- Container vulnerability scanning
- Network policy auditing
- RBAC configuration validation
- Secrets management audit

**Compliance Standards**:
- CIS Kubernetes Benchmark
- Container security best practices
- Network segmentation validation
- Privilege escalation prevention

**Usage**:
```bash
kubectl apply -f security-compliance-scan.yaml
kubectl logs -n security-testing job/security-compliance-scanner -f
```

## Test Execution Workflow

### 1. Initial Setup

```bash
# Create testing namespaces
kubectl apply -f penetration-testing.yaml
kubectl apply -f chaos-engineering.yaml

# Verify namespace creation
kubectl get namespaces security-testing chaos-engineering
```

### 2. Run Security Tests

```bash
# Execute penetration testing
kubectl create job --from=cronjob/penetration-test-runner penetration-test-$(date +%s) -n security-testing

# Execute compliance scanning
kubectl create job --from=cronjob/security-compliance-scanner compliance-scan-$(date +%s) -n security-testing
```

### 3. Run Performance Tests

```bash
# Execute load testing
kubectl create job --from=cronjob/load-test-runner load-test-$(date +%s) -n security-testing

# Execute stress testing
kubectl create job --from=cronjob/stress-test-runner stress-test-$(date +%s) -n security-testing
```

### 4. Run Chaos Tests

```bash
# Execute chaos engineering tests
kubectl create job --from=cronjob/chaos-pod-killer chaos-pod-$(date +%s) -n chaos-engineering
kubectl create job --from=cronjob/chaos-network-test chaos-network-$(date +%s) -n chaos-engineering
kubectl create job --from=cronjob/chaos-resource-exhaustion chaos-resource-$(date +%s) -n chaos-engineering
```

### 5. Monitor Results

```bash
# Check test pod status
kubectl get pods -n security-testing
kubectl get pods -n chaos-engineering

# View test logs
kubectl logs -n security-testing -l component=security-testing
kubectl logs -n chaos-engineering -l component=chaos-engineering

# Extract test results
kubectl exec -n security-testing <test-pod> -- cat /results/test-results.json
```

## Expected Test Results

### Security Tests
- **Penetration Testing**: Should validate that security controls block unauthorized access
- **Compliance Scanning**: Should confirm adherence to security standards
- **Container Security**: Should verify sandbox isolation and resource limits

### Performance Tests
- **Load Testing**: Should demonstrate stable performance under normal load
- **Stress Testing**: Should validate graceful degradation under extreme load
- **Autoscaling**: Should verify HPA/VPA response to load changes

### Resilience Tests
- **Pod Failures**: Should demonstrate automatic recovery and continued service
- **Network Issues**: Should validate connection resilience and retry mechanisms
- **Resource Exhaustion**: Should confirm resource limits prevent system disruption

## Test Metrics and Thresholds

### Performance Thresholds
- **Response Time**: 95th percentile < 2 seconds
- **Error Rate**: < 5% under normal load, < 10% under stress
- **Throughput**: Minimum 100 submissions/minute sustained
- **Recovery Time**: < 30 seconds for pod failures

### Security Validation
- **Container Escape**: 0 successful escape attempts
- **Resource Limits**: All limits enforced and respected
- **Network Isolation**: Unauthorized network access blocked
- **Privilege Escalation**: All escalation attempts blocked

### Compliance Requirements
- **CIS Benchmark**: Score > 80%
- **Vulnerability Scanning**: No critical vulnerabilities
- **Network Policies**: All required policies in place
- **RBAC**: Principle of least privilege enforced

## Automated Scheduling

The testing suite includes automated scheduling for continuous validation:

- **Daily Load Tests**: 2 AM UTC
- **Weekly Compliance Scans**: Sunday 1 AM UTC  
- **Weekly Chaos Tests**: Monday 3 AM UTC

## Integration with Monitoring

Test results are integrated with the monitoring system:

- Metrics exported to Prometheus
- Alerts configured for test failures
- Dashboards showing test trends
- Historical performance tracking

## Troubleshooting

### Common Issues

1. **Permission Errors**: Ensure service accounts have required RBAC permissions
2. **Resource Constraints**: Verify sufficient cluster resources for testing
3. **Network Connectivity**: Check network policies allow test pod communication
4. **Tool Installation**: Verify container images have required testing tools

### Debug Commands

```bash
# Check test pod events
kubectl describe pod <test-pod> -n <namespace>

# View detailed logs
kubectl logs <test-pod> -n <namespace> --previous

# Execute interactive shell for debugging
kubectl exec -it <test-pod> -n <namespace> -- /bin/sh

# Check resource usage
kubectl top pods -n <namespace>
```

This comprehensive testing suite ensures the enhanced judge system meets security, performance, and reliability requirements in production environments.