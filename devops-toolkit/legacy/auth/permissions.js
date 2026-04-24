/**
 * Permission System for Device Management
 * Based on PERMISSION_MODEL.md design
 */

const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

// Load permissions configuration
function loadPermissionsConfig() {
  const configPath = path.join(__dirname, '../config/permissions.yaml');
  try {
    const fileContents = fs.readFileSync(configPath, 'utf8');
    return yaml.load(fileContents);
  } catch (e) {
    console.error('Failed to load permissions config:', e.message);
    return null;
  }
}

const permissionsConfig = loadPermissionsConfig();

/**
 * Permission Result
 */
class PermissionResult {
  constructor(allowed, reason, requiredRole = null) {
    this.allowed = allowed;
    this.reason = reason;
    this.requiredRole = requiredRole;
  }

  static allow() {
    return new PermissionResult(true, 'allowed');
  }

  static deny(reason) {
    return new PermissionResult(false, reason);
  }
}

/**
 * Check if a user role has a specific permission
 */
function hasPermission(role, permission) {
  if (!permissionsConfig || !permissionsConfig.group_mappings) {
    return false;
  }

  // Find the role in group mappings
  for (const [groupName, mapping] of Object.entries(permissionsConfig.group_mappings)) {
    if (mapping.system_role === role) {
      if (mapping.permissions && mapping.permissions.includes(permission)) {
        return true;
      }
      if (mapping.permissions && mapping.permissions.includes('admin')) {
        return true; // admin has all permissions
      }
    }
  }

  return false;
}

/**
 * Get the required role for an operation
 */
function getRequiredRole(operation) {
  if (!permissionsConfig || !permissionsConfig.operations) {
    return null;
  }

  const op = permissionsConfig.operations[operation];
  return op ? op.requires_role : null;
}

/**
 * Check if a user's roles allow an operation on a device with given labels
 */
function checkDevicePermission(userRoles, operation, deviceLabels = []) {
  if (!userRoles || userRoles.length === 0) {
    return PermissionResult.deny('No user roles provided');
  }

  // Check if operation exists
  const requiredRole = getRequiredRole(operation);
  if (!requiredRole) {
    // Operation doesn't exist in config, allow by default
    return PermissionResult.allow();
  }

  // Check if user has the required role
  const userHasRequiredRole = userRoles.some(role => {
    if (role === requiredRole) return true;
    // SuperAdmin has all permissions
    if (role === 'SuperAdmin') return true;
    // Operator has Operator+ permissions
    if (role === 'Operator' && (requiredRole === 'Operator' || requiredRole === 'Developer')) return true;
    return false;
  });

  if (!userHasRequiredRole) {
    return PermissionResult.deny(
      `Operation '${operation}' requires '${requiredRole}' role`,
      requiredRole
    );
  }

  // Check environment restrictions
  if (permissionsConfig.operations[operation].env_restrictions) {
    const envLabels = deviceLabels.filter(l => l.startsWith('env:'));
    for (const envLabel of envLabels) {
      const restriction = permissionsConfig.operations[operation].env_restrictions[envLabel];
      if (restriction) {
        const userHasEnvPermission = userRoles.some(role =>
          role === restriction || role === 'SuperAdmin'
        );
        if (!userHasEnvPermission) {
          return PermissionResult.deny(
            `Operation '${operation}' on '${envLabel}' requires '${restriction}' role`,
            restriction
          );
        }
      }
    }
  }

  return PermissionResult.allow();
}

/**
 * Check if user can access device based on labels
 */
function checkDeviceAccess(userRoles, deviceLabels = []) {
  if (!userRoles || userRoles.length === 0) {
    return PermissionResult.deny('No user roles provided');
  }

  // SuperAdmin can access everything
  if (userRoles.includes('SuperAdmin')) {
    return PermissionResult.allow();
  }

  // Check business hierarchy for inherited access
  const bizLabels = deviceLabels.filter(l => l.startsWith('biz:'));

  for (const bizLabel of bizLabels) {
    // Check if user has direct access to this business unit
    const hasDirectAccess = checkBusinessAccess(userRoles, bizLabel);
    if (hasDirectAccess) {
      return PermissionResult.allow();
    }

    // Check if user has access to parent business units
    const parentAccess = checkParentBusinessAccess(userRoles, bizLabel);
    if (parentAccess) {
      return PermissionResult.allow();
    }
  }

  // Check environment-based access
  const envLabels = deviceLabels.filter(l => l.startsWith('env:'));
  for (const envLabel of envLabels) {
    const hasEnvAccess = checkEnvAccess(userRoles, envLabel);
    if (hasEnvAccess) {
      return PermissionResult.allow();
    }
  }

  return PermissionResult.deny('No matching access rules found');
}

/**
 * Check if user roles allow access to a specific business unit
 */
function checkBusinessAccess(userRoles, bizLabel) {
  if (!permissionsConfig || !permissionsConfig.label_permissions) {
    return false;
  }

  const labelPerms = permissionsConfig.label_permissions[bizLabel];
  if (!labelPerms || !labelPerms.groups) {
    return false;
  }

  // Map user roles to allowed groups
  const roleToGroups = {
    'Operator': ['ops:admin', 'ops:prod', 'ops:core', 'ops:payments'],
    'Developer': ['dev:admin', 'dev:ops'],
    'Auditor': ['audit:read'],
    'SuperAdmin': ['ops:admin', 'SuperAdmin']
  };

  for (const role of userRoles) {
    const allowedGroups = roleToGroups[role] || [];
    const hasAccess = labelPerms.groups.some(g => allowedGroups.includes(g));
    if (hasAccess) return true;
  }

  return false;
}

/**
 * Check if user has access to parent business units (inheritance)
 */
function checkParentBusinessAccess(userRoles, bizLabel) {
  if (!permissionsConfig || !permissionsConfig.business_hierarchy) {
    return false;
  }

  // Find if bizLabel is a child and check parent access
  for (const [parent, config] of Object.entries(permissionsConfig.business_hierarchy)) {
    if (config.children && config.children.includes(bizLabel)) {
      // Check if user has access to parent
      if (checkBusinessAccess(userRoles, parent)) {
        return true;
      }
    }
  }

  return false;
}

/**
 * Check environment-based access
 */
function checkEnvAccess(userRoles, envLabel) {
  if (!permissionsConfig || !permissionsConfig.label_permissions) {
    return false;
  }

  const labelPerms = permissionsConfig.label_permissions[envLabel];
  if (!labelPerms || !labelPerms.groups) {
    return false;
  }

  const roleToGroups = {
    'Operator': ['ops:admin', 'ops:prod'],
    'Developer': ['dev:admin', 'dev:ops'],
    'Auditor': [],
    'SuperAdmin': ['ops:admin', 'SuperAdmin']
  };

  for (const role of userRoles) {
    const allowedGroups = roleToGroups[role] || [];
    const hasAccess = labelPerms.groups.some(g => allowedGroups.includes(g));
    if (hasAccess) return true;
  }

  return false;
}

/**
 * Get all permissions for a role
 */
function getPermissionsForRole(role) {
  if (!permissionsConfig || !permissionsConfig.group_mappings) {
    return [];
  }

  for (const [groupName, mapping] of Object.entries(permissionsConfig.group_mappings)) {
    if (mapping.system_role === role) {
      return mapping.permissions || [];
    }
  }

  return [];
}

module.exports = {
  checkDevicePermission,
  checkDeviceAccess,
  hasPermission,
  getPermissionsForRole,
  getRequiredRole,
  PermissionResult,
  permissionsConfig
};
