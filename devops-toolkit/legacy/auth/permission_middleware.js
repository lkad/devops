/**
 * Permission Middleware
 * Enforces permission checks on API routes based on user roles and device labels
 */

const {
  checkDevicePermission,
  checkDeviceAccess,
  hasPermission,
  getRequiredRole,
  getPermissionsForRole,
  PermissionResult
} = require('./permissions');

/**
 * Create permission middleware for device operations
 */
function requirePermission(operation) {
  return (req, res, next) => {
    const userRoles = req.user?.roles || ['SuperAdmin']; // Default to SuperAdmin for development

    // Check if user has the required permission
    const result = checkDevicePermission(userRoles, operation);

    if (!result.allowed) {
      return res.status(403).json({
        success: false,
        error: 'Permission denied',
        reason: result.reason,
        requiredRole: result.requiredRole
      });
    }

    next();
  };
}

/**
 * Middleware to check device-specific permissions
 * Must be used after device is loaded (adds req.device)
 */
function requireDevicePermission(operation) {
  return async (req, res, next) => {
    const userRoles = req.user?.roles || ['SuperAdmin'];
    const device = req.device;

    if (!device) {
      return res.status(400).json({
        success: false,
        error: 'Device not loaded'
      });
    }

    // Extract labels from device
    const labels = device.labels || [];
    const labelStrings = labels.map(l => {
      if (typeof l === 'object') {
        return Object.entries(l).map(([k, v]) => `${k}:${v}`).join(',');
      }
      return String(l);
    });

    // Check permission for operation
    const permResult = checkDevicePermission(userRoles, operation, labelStrings);
    if (!permResult.allowed) {
      return res.status(403).json({
        success: false,
        error: 'Permission denied',
        reason: permResult.reason,
        requiredRole: permResult.requiredRole
      });
    }

    // Check device access based on labels
    const accessResult = checkDeviceAccess(userRoles, labelStrings);
    if (!accessResult.allowed) {
      return res.status(403).json({
        success: false,
        error: 'Device access denied',
        reason: accessResult.reason
      });
    }

    next();
  };
}

/**
 * Middleware to load device by ID and check permissions
 */
function loadDeviceAndCheckPermission(operation) {
  return async (req, res, next) => {
    const deviceId = req.params.id || req.params.deviceId;
    if (!deviceId) {
      return res.status(400).json({
        success: false,
        error: 'Device ID required'
      });
    }

    // DeviceManager should be available on req.app or as a dependency
    const deviceManager = req.app?.deviceManager;
    if (!deviceManager) {
      console.error('DeviceManager not available on req.app');
      return res.status(500).json({
        success: false,
        error: 'Server configuration error'
      });
    }

    const device = deviceManager.getDevice(deviceId);
    if (!device) {
      return res.status(404).json({
        success: false,
        error: 'Device not found'
      });
    }

    req.device = device;
    next();
  };
}

/**
 * Role-based access control middleware
 */
function requireRole(role) {
  return (req, res, next) => {
    const userRoles = req.user?.roles || [];

    if (userRoles.includes('SuperAdmin') || userRoles.includes(role)) {
      return next();
    }

    return res.status(403).json({
      success: false,
      error: 'Insufficient role',
      required: role,
      current: userRoles
    });
  };
}

/**
 * Check if user has any of the specified roles
 */
function requireAnyRole(roles) {
  return (req, res, next) => {
    const userRoles = req.user?.roles || [];

    const hasRole = userRoles.some(r =>
      r === 'SuperAdmin' || roles.includes(r)
    );

    if (!hasRole) {
      return res.status(403).json({
        success: false,
        error: 'Insufficient role',
        required: roles,
        current: userRoles
      });
    }

    next();
  };
}

/**
 * Attach user to request (for development/testing)
 * In production, this would validate JWT or session
 */
function attachUser(user = { roles: ['SuperAdmin'] }) {
  return (req, res, next) => {
    req.user = user;
    next();
  };
}

/**
 * Get user's effective permissions
 */
function getUserPermissions(req) {
  const userRoles = req.user?.roles || [];
  const allPerms = {};

  for (const role of userRoles) {
    const perms = getPermissionsForRole(role);
    for (const perm of perms) {
      allPerms[perm] = true;
    }
  }

  return allPerms;
}

module.exports = {
  requirePermission,
  requireDevicePermission,
  loadDeviceAndCheckPermission,
  requireRole,
  requireAnyRole,
  attachUser,
  getUserPermissions,
  checkDevicePermission,
  checkDeviceAccess,
  hasPermission,
  getRequiredRole,
  getPermissionsForRole
};
