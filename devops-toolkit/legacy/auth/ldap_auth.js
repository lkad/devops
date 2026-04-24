/**
 * LDAP Authentication Module
 * Provides user authentication and group-to-role mapping
 */

const ldap = require('ldapjs');
const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

// Load LDAP configuration
function loadLdapConfig() {
  const configPath = path.join(__dirname, '../config/ldap.yaml');
  try {
    if (fs.existsSync(configPath)) {
      const fileContents = fs.readFileSync(configPath, 'utf8');
      return yaml.load(fileContents);
    }
  } catch (e) {
    console.error('Failed to load LDAP config:', e.message);
  }
  return getDefaultConfig();
}

function getDefaultConfig() {
  return {
    url: process.env.LDAP_URL || 'ldap://localhost:389',
    bind_dn: process.env.LDAP_BIND_DN || 'cn=admin,dc=example,dc=com',
    bind_password: process.env.LDAP_BIND_PASSWORD || 'admin',
    search_base: process.env.LDAP_SEARCH_BASE || 'dc=example,dc=com',
    user_search_filter: '(uid={{username}})',
    group_search_base: process.env.LDAP_GROUP_SEARCH_BASE || 'ou=Groups,dc=example,dc=com',
    group_search_filter: '(member={{user_dn}})',
    role_mapping: {
      'cn=IT_Ops,ou=Groups,dc=example,dc=com': 'Operator',
      'cn=DevTeam_Payments,ou=Groups,dc=example,dc=com': 'Developer',
      'cn=Security_Auditors,ou=Groups,dc=example,dc=com': 'Auditor',
      'cn=SRE_Lead,ou=Groups,dc=example,dc=com': 'SuperAdmin'
    }
  };
}

const config = loadLdapConfig();

// Connection pool
let pool = [];
const MAX_POOL_SIZE = 10;

/**
 * Get a connection from the pool
 */
function getConnection() {
  if (pool.length > 0) {
    return pool.pop();
  }
  return ldap.createClient({
    url: config.url,
    connectTimeout: 5000,
    timeout: 10000
  });
}

/**
 * Return a connection to the pool
 */
function returnConnection(client) {
  if (pool.length < MAX_POOL_SIZE) {
    pool.push(client);
  } else {
    client.unbind();
  }
}

/**
 * Authenticate user against LDAP
 */
async function authenticate(username, password) {
  if (!username || !password) {
    return { success: false, error: 'Username and password required' };
  }

  const client = getConnection();
  try {
    // First bind with service account
    await bindClient(client, config.bind_dn, config.bind_password);

    // Search for user
    const user = await searchUser(client, username);
    if (!user) {
      return { success: false, error: 'User not found' };
    }

    // Attempt to bind as user to verify password
    try {
      await bindClient(client, user.dn, password);
    } catch (bindErr) {
      return { success: false, error: 'Invalid credentials' };
    }

    // Get user groups
    const groups = await getUserGroups(client, user.dn);

    // Map groups to roles
    const roles = mapGroupsToRoles(groups);

    return {
      success: true,
      user: {
        username: user.uid || user.cn || username,
        dn: user.dn,
        groups,
        roles
      }
    };
  } catch (err) {
    console.error('LDAP auth error:', err.message);
    return { success: false, error: 'Authentication failed' };
  } finally {
    returnConnection(client);
  }
}

/**
 * Bind LDAP client with credentials
 */
function bindClient(client, dn, password) {
  return new Promise((resolve, reject) => {
    client.bind(dn, password, (err) => {
      if (err) reject(err);
      else resolve();
    });
  });
}

/**
 * Search for user in LDAP directory
 */
function searchUser(client, username) {
  return new Promise((resolve, reject) => {
    const filter = config.user_search_filter.replace('{{username}}', username);
    const opts = {
      filter,
      scope: 'sub',
      attributes: ['dn', 'uid', 'cn', 'mail', 'memberOf']
    };

    client.search(config.search_base, opts, (err, res) => {
      if (err) {
        reject(err);
        return;
      }

      let user = null;
      res.on('searchEntry', (entry) => {
        user = {
          dn: entry.objectName,
          uid: entry.attributes.find(a => a.type === 'uid')?.values[0],
          cn: entry.attributes.find(a => a.type === 'cn')?.values[0],
          mail: entry.attributes.find(a => a.type === 'mail')?.values[0],
          memberOf: entry.attributes.find(a => a.type === 'memberOf')?.values || []
        };
      });

      res.on('error', (err) => reject(err));
      res.on('end', () => resolve(user));
    });
  });
}

/**
 * Get groups for a user
 */
async function getUserGroups(client, userDn) {
  return new Promise((resolve, reject) => {
    const filter = config.group_search_filter.replace('{{user_dn}}', userDn);
    const opts = {
      filter,
      scope: 'sub',
      attributes: ['cn', 'description']
    };

    client.search(config.group_search_base, opts, (err, res) => {
      if (err) {
        reject(err);
        return;
      }

      const groups = [];
      res.on('searchEntry', (entry) => {
        groups.push(entry.objectName);
      });

      res.on('error', (err) => reject(err));
      res.on('end', () => resolve(groups));
    });
  });
}

/**
 * Map LDAP groups to system roles
 */
function mapGroupsToRoles(groups) {
  const roles = new Set();

  for (const group of groups) {
    // Direct mapping
    if (config.role_mapping && config.role_mapping[group]) {
      roles.add(config.role_mapping[group]);
    }

    // Check if any mapping key is a suffix of the group
    for (const [groupPattern, role] of Object.entries(config.role_mapping || {})) {
      if (group.includes(groupPattern) || groupPattern.includes(group)) {
        roles.add(role);
      }
    }
  }

  return Array.from(roles);
}

/**
 * Get groups for a user (LDAP API)
 */
async function getGroups(username) {
  const client = getConnection();
  try {
    await bindClient(client, config.bind_dn, config.bind_password);
    const user = await searchUser(client, username);
    if (!user) {
      return { success: false, error: 'User not found' };
    }
    const groups = await getUserGroups(client, user.dn);
    return { success: true, groups };
  } catch (err) {
    return { success: false, error: err.message };
  } finally {
    returnConnection(client);
  }
}

/**
 * Get roles for a user (LDAP API)
 */
async function getRoles(username) {
  const groupsResult = await getGroups(username);
  if (!groupsResult.success) {
    return groupsResult;
  }
  const roles = mapGroupsToRoles(groupsResult.groups);
  return { success: true, roles };
}

/**
 * Health check for LDAP connection
 */
async function healthCheck() {
  const client = getConnection();
  try {
    await bindClient(client, config.bind_dn, config.bind_password);
    return { success: true, message: 'LDAP connection healthy' };
  } catch (err) {
    return { success: false, error: err.message };
  } finally {
    returnConnection(client);
  }
}

/**
 * Create LDAP auth middleware for Express (if needed in future)
 */
function createAuthMiddleware() {
  return async (req, res, next) => {
    // For now, skip auth if no credentials provided
    // In production, implement JWT/session-based auth
    const authHeader = req.headers.authorization;
    if (!authHeader || !authHeader.startsWith('Basic ')) {
      req.user = { roles: ['SuperAdmin'] }; // Default for development
      return next();
    }

    const base64Credentials = authHeader.split(' ')[1];
    const credentials = Buffer.from(base64Credentials, 'base64').toString('utf8');
    const [username, password] = credentials.split(':');

    const result = await authenticate(username, password);
    if (result.success) {
      req.user = result.user;
      next();
    } else {
      res.status(401).json({ success: false, error: result.error });
    }
  };
}

function getPoolStats() {
  return {
    size: pool.length,
    available: pool.length
  };
}

module.exports = {
  authenticate,
  getGroups,
  getRoles,
  healthCheck,
  createAuthMiddleware,
  config,
  mapGroupsToRoles,
  getPoolStats
};
