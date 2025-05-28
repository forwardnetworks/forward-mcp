# Troubleshooting Guide

## TLS Certificate Issues

### Problem: `tls: failed to verify certificate: x509: certificate signed by unknown authority`

This error occurs when the Forward Networks instance uses a self-signed certificate or an internal CA that your system doesn't trust.

**Solutions:**

1. **Skip Certificate Verification (Development Only)**
   ```env
   FORWARD_INSECURE_SKIP_VERIFY=true
   ```
   ⚠️ **Security Warning**: Only use in development or controlled environments.

2. **Add Custom CA Certificate**
   ```env
   FORWARD_CA_CERT_PATH=/path/to/ca-certificate.pem
   ```

3. **System-wide CA Installation** (Alternative)
   ```bash
   # macOS
   sudo security add-trusted-cert -d -r trustRoot -k /System/Library/Keychains/SystemRootCertificates.keychain ca-cert.pem
   
   # Linux (Ubuntu/Debian)
   sudo cp ca-cert.pem /usr/local/share/ca-certificates/forward-ca.crt
   sudo update-ca-certificates
   ```

### Problem: `tls: failed to verify certificate: x509: certificate is valid for wrong-hostname`

This occurs when the certificate doesn't match the hostname you're connecting to.

**Solutions:**

1. **Use the correct hostname** that matches the certificate
2. **Skip verification** (development only):
   ```env
   FORWARD_INSECURE_SKIP_VERIFY=true
   ```

### Problem: `tls: failed to verify certificate: x509: certificate has expired`

**Solutions:**

1. **Contact your Forward Networks administrator** to renew the certificate
2. **Temporary workaround** (development only):
   ```env
   FORWARD_INSECURE_SKIP_VERIFY=true
   ```

## Authentication Issues

### Problem: `HTTP 401 Unauthorized`

**Solutions:**

1. **Verify API credentials**:
   ```env
   FORWARD_API_KEY=your-correct-api-key
   FORWARD_API_SECRET=your-correct-api-secret
   ```

2. **Check API key permissions** with your Forward Networks administrator

3. **Verify API endpoint URL**:
   ```env
   FORWARD_API_BASE_URL=https://your-forward-instance.com
   ```

### Problem: `HTTP 403 Forbidden`

Your API key is valid but lacks permissions for the requested operation.

**Solutions:**

1. **Contact your Forward Networks administrator** to grant appropriate permissions
2. **Verify the network ID** you're trying to access exists and you have access

## Network Connectivity Issues

### Problem: `no such host` or `connection timeout`

**Solutions:**

1. **Verify the Forward Networks URL**:
   ```bash
   ping your-forward-instance.com
   curl -I https://your-forward-instance.com
   ```

2. **Check network connectivity** and firewall rules

3. **Increase timeout** for slow networks:
   ```env
   FORWARD_TIMEOUT=60
   ```

### Problem: `connection refused`

**Solutions:**

1. **Verify the port** (typically 443 for HTTPS)
2. **Check if the Forward Networks service is running**
3. **Verify firewall rules** allow outbound HTTPS traffic

## Configuration Issues

### Problem: Environment variables not loading

**Solutions:**

1. **Verify .env file location** (should be in project root)
2. **Check .env file format**:
   ```env
   # Correct format (no spaces around =)
   FORWARD_API_KEY=your-key
   
   # Incorrect format
   FORWARD_API_KEY = your-key
   ```

3. **Verify file permissions**:
   ```bash
   chmod 600 .env
   ```

### Problem: Claude Desktop not finding the server

**Solutions:**

1. **Verify the binary path** in `claude_desktop_config.json`:
   ```json
   {
     "mcpServers": {
       "forward-networks": {
         "command": "/absolute/path/to/forward-mcp-server"
       }
     }
   }
   ```

2. **Make sure the binary is executable**:
   ```bash
   chmod +x /path/to/forward-mcp-server
   ```

3. **Check Claude Desktop logs** for error messages

## API Response Issues

### Problem: `json: cannot unmarshal array into Go value of type X`

This indicates a mismatch between expected and actual API response format.

**Solutions:**

1. **Check your Forward Networks version** - API responses may vary between versions
2. **Contact support** if the issue persists
3. **Enable debug logging** to see raw API responses

### Problem: `unexpected status code: 400`

**Solutions:**

1. **Verify request parameters** (network IDs, query syntax, etc.)
2. **Check NQE query syntax** if using NQE tools
3. **Verify the snapshot exists** if specifying a snapshot ID

## Performance Issues

### Problem: Slow API responses

**Solutions:**

1. **Increase timeout**:
   ```env
   FORWARD_TIMEOUT=120
   ```

2. **Reduce result limits** for large queries:
   ```bash
   # In Claude Desktop, ask for smaller result sets
   "List first 10 devices in network ABC"
   ```

3. **Use specific snapshots** instead of latest:
   ```env
   # Specify snapshot ID in requests
   ```

## Debugging Tips

### Enable Verbose Logging

1. **Set environment variable**:
   ```env
   DEBUG=true
   LOG_LEVEL=debug
   ```

2. **Run with verbose output**:
   ```bash
   ./bin/forward-mcp-server --verbose
   ```

### Test API Connectivity

Use the test runner to verify connectivity:

```bash
# Test with integration tests (requires .env)
./scripts/test.sh integration

# Check specific tools
./scripts/test.sh unit -v
```

### Manual API Testing

```bash
# Test basic connectivity
curl -u "api-key:api-secret" https://your-forward-instance.com/api/networks

# Test with custom CA
curl --cacert /path/to/ca.pem -u "api-key:api-secret" https://your-forward-instance.com/api/networks

# Test skipping verification (development only)
curl -k -u "api-key:api-secret" https://your-forward-instance.com/api/networks
```

## Common Configuration Examples

### Development Environment (Self-Signed Certs)

```env
FORWARD_API_KEY=dev-api-key
FORWARD_API_SECRET=dev-api-secret
FORWARD_API_BASE_URL=https://forward-dev.internal
FORWARD_INSECURE_SKIP_VERIFY=true
FORWARD_TIMEOUT=30
```

### Production Environment (Internal CA)

```env
FORWARD_API_KEY=prod-api-key
FORWARD_API_SECRET=prod-api-secret
FORWARD_API_BASE_URL=https://forward.company.com
FORWARD_CA_CERT_PATH=/etc/ssl/certs/company-ca.pem
FORWARD_TIMEOUT=60
```

### High-Security Environment (Mutual TLS)

```env
FORWARD_API_KEY=secure-api-key
FORWARD_API_SECRET=secure-api-secret
FORWARD_API_BASE_URL=https://forward-secure.company.com
FORWARD_CA_CERT_PATH=/etc/ssl/certs/company-ca.pem
FORWARD_CLIENT_CERT_PATH=/etc/ssl/certs/forward-client.pem
FORWARD_CLIENT_KEY_PATH=/etc/ssl/private/forward-client.key
FORWARD_TIMEOUT=45
```

## Getting Help

1. **Check the logs** in Claude Desktop console
2. **Run integration tests** to isolate the issue
3. **Test manual API calls** with curl
4. **Contact your Forward Networks administrator** for server-side issues
5. **Open a GitHub issue** for suspected bugs

## Quick Diagnostic Checklist

- [ ] API credentials are correct
- [ ] Forward Networks URL is accessible
- [ ] Network connectivity is working
- [ ] TLS/SSL configuration is appropriate
- [ ] .env file is properly formatted
- [ ] Binary has execute permissions
- [ ] Claude Desktop config path is correct
- [ ] Forward Networks service is running 