// config/config.js - 主配置文件
module.exports = {
  env: process.env.ENV || 'development',
  
  // 环境配置
  environment: {
    development: {
      port: 3000,
      debug: true
    },
    test: {
      port: 3001,
      debug: false
    },
    production: {
      port: 8080,
      debug: false
    }
  },
  
  // AD/LDAP 认证配置
  authentication: {
    ldap: {
      uri: process.env.LDAP_URI || 'dc=example,dc=com',
      baseDN: process.env.LDAP_BASE_DN || 'ou=users,dc=example,dc=com',
      bind: process.env.LDAP_BIND,
      timeout: 30000
    },
    sso: {
      enabled: process.env.SSO_ENABLED !== 'false',
      provider: process.env.SSO_PROVIDER || 'google',
      issuer: process.env.SSO_ISSUER
    },
    mfa: {
      enabled: process.env.MFA_ENABLED !== 'false',
      type: process.env.MFA_TYPE || 'totp'
    },
    sessions: {
      jwt_expiry_minutes: 15,
      refresh_expiry_days: 7,
      remember_me_days: 30
    }
  },
  
  // RBAC 角色配置
  authorization: {
    roles: {
      admin: {
        permissions: ['read', 'write', 'execute', 'admin']
      },
      operator: {
        permissions: ['read', 'write', 'execute']
      },
      developer: {
        permissions: ['read', 'test-deploy']
      },
      auditor: {
        permissions: ['read']
      }
    }
  },
  
  // 标签组权限配置
  label_permissions: {
    'env:prod': ['ops:admin', 'ops:prod'],
    'env:dev': ['dev:admin', 'dev:ops'],
    'env:test': ['dev:ops'],
    'biz:core': ['ops:admin', 'ops:core'],
    'biz:payments': ['ops:admin', 'ops:payments']
  },
  
  // 业务组继承关系
  business_hierarchy: {
    'biz:core': {
      children: ['biz:core-api', 'biz:core-infrastructure']
    },
    'biz:payments': {
      parents: ['biz:core']
    }
  },
  
  // 设备配置
  devices: {
    default_timeout_ms: 30000,
    heartbeat_interval_seconds: 30,
    retry_interval_ms: 5000,
    max_retries: 3
  },
  
  // 日志配置
  logging: {
    level: process.env.LOG_LEVEL || 'info',
    format: 'json',
    transporters: ['console', 'file']
  },
  
  // DevOps 服务配置
  devops: {
    agent_port: 8081,
    sync_interval_ms: 1000,
    healthcheck_interval_seconds: 10
  }
};
