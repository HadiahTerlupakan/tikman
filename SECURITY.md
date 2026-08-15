# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Currently supported versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

We take the security of TikMan OLT Provisioning System seriously. If you believe you have found a security vulnerability, please report it to us as described below.

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please report them via email to: [security@your-domain.com]

You should receive a response within 48 hours. If for some reason you do not, please follow up via email to ensure we received your original message.

Please include the following information:

- Type of issue (e.g. buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit it

## Security Features

### Authentication & Authorization

- **Session-based authentication** with HTTP-only cookies
- **Redis session storage** with 24-hour TTL
- **Role-Based Access Control (RBAC)** with 3 levels:
  - Admin (full access)
  - Technician (read/write sites and OLTs)
  - Viewer (read-only access)
- **Automatic session refresh** on activity
- **Secure logout** mechanism

### Data Protection

- **Password hashing** using bcrypt with cost factor 12
- **OLT credentials encryption** using AES-256-GCM
- **Session tokens** stored securely in Redis
- **Environment variables** for all secrets (never hardcoded)

### Security Headers

Production deployments include:
- `X-Frame-Options: DENY` - Prevent clickjacking
- `X-Content-Type-Options: nosniff` - Prevent MIME sniffing
- `X-XSS-Protection: 1; mode=block` - XSS protection
- `Strict-Transport-Security` - Force HTTPS (production only)
- `Content-Security-Policy` - Control resource loading
- `Referrer-Policy: strict-origin-when-cross-origin` - Limit referrer info

### Input Validation

All user inputs are validated:
- Username: 3-50 alphanumeric characters
- Email: Valid email format, max 255 characters
- Password: Minimum 12 characters (8 for login compatibility)
- IP Address: Valid IPv4 format
- Ports: 1-65535 range
- Role: Enum validation (admin, technician, viewer)

### Rate Limiting

- **100 requests per minute per IP** (configurable)
- Applied globally to all endpoints
- Automatic cleanup of old entries

### SQL Injection Protection

- **GORM ORM** with parameterized queries
- No raw SQL concatenation
- Automatic input escaping

### CORS Configuration

- **Environment-based allowed origins**
- Credentials support for cookies
- Specific HTTP methods allowed
- Specific headers allowed

## Security Best Practices for Deployment

### Production Environment

1. **Generate Strong Secrets**
   ```bash
   # SESSION_SECRET (32 bytes)
   openssl rand -base64 32
   
   # ENCRYPTION_KEY (32 bytes hex)
   openssl rand -hex 16
   
   # Database passwords
   openssl rand -base64 32
   ```

2. **Environment Variables**
   ```bash
   ENVIRONMENT=production
   ALLOWED_ORIGINS=https://your-domain.com
   DB_PASSWORD=<strong-random-password>
   REDIS_PASSWORD=<strong-random-password>
   ENCRYPTION_KEY=<32-byte-hex-string>
   SESSION_SECRET=<strong-random-string>
   ```

3. **HTTPS Configuration**
   - Use valid SSL/TLS certificates (Let's Encrypt recommended)
   - Enable HSTS headers (automatic in production mode)
   - Redirect HTTP to HTTPS (automatic in production mode)

4. **Database Security**
   - Use strong passwords
   - Enable SSL/TLS for database connections
   - Restrict database access to application servers only
   - Regular backups with encryption

5. **Redis Security**
   - Use strong password (requirepass)
   - Bind to localhost or private network only
   - Disable dangerous commands
   - Use TLS for connections in production

6. **Network Security**
   - Use firewall rules to restrict access
   - Deploy behind reverse proxy (nginx/caddy)
   - Use VPN for administrative access
   - Enable DDoS protection

7. **Monitoring & Logging**
   - Enable audit logging
   - Monitor failed login attempts
   - Set up alerts for suspicious activities
   - Regular security log reviews

### Default Credentials

**IMPORTANT:** Change the default admin credentials immediately after first deployment.

Default credentials (development only):
- Username: `admin`
- Password: `admin123`

**Production:** Force password change on first login or generate random password during deployment.

## Security Checklist

### Pre-Production

- [ ] Generate strong random secrets for production
- [ ] Change default admin password
- [ ] Enable HTTPS with valid SSL certificate
- [ ] Configure CORS with production domain
- [ ] Set ENVIRONMENT=production
- [ ] Enable PostgreSQL SSL connection
- [ ] Review and update rate limiting
- [ ] Setup log aggregation and monitoring
- [ ] Implement alerting for security events
- [ ] Backup and test disaster recovery procedures

### Regular Maintenance

- [ ] Update dependencies regularly
- [ ] Run security scans (weekly)
- [ ] Review audit logs (daily)
- [ ] Rotate secrets (quarterly)
- [ ] Review user access (monthly)
- [ ] Test backup restoration (monthly)
- [ ] Security audit (annually)
- [ ] Penetration testing (annually)

## Vulnerability Disclosure Timeline

1. **Day 0:** Security issue reported
2. **Day 1-2:** Initial response and triage
3. **Day 3-7:** Investigation and fix development
4. **Day 8-14:** Testing and verification
5. **Day 15:** Patch release and disclosure

## Security Tools Integration

### Recommended Tools

- **Trivy** - Container vulnerability scanning
- **GoSec** - Go security scanner
- **npm audit** - Frontend dependency scanning
- **OWASP ZAP** - Dynamic application security testing
- **SonarQube** - Static code analysis

### CI/CD Security Checks

Our CI/CD pipeline includes:
- Dependency vulnerability scanning
- Static security analysis
- Container image scanning
- Secret detection
- License compliance checks

## Contact

For security-related questions or concerns, contact:
- Email: security@your-domain.com
- Security Team: [Your Organization Name]

## Acknowledgments

We would like to thank the following security researchers for responsibly disclosing vulnerabilities:

- [To be added when applicable]

## Additional Resources

- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE Top 25](https://cwe.mitre.org/top25/)
- [NIST Cybersecurity Framework](https://www.nist.gov/cyberframework)
