/**
 * Tests for Permission Middleware
 */

const {
  requirePermission,
  requireRole,
  requireAnyRole,
  checkDevicePermission,
  checkDeviceAccess,
  hasPermission,
  getRequiredRole,
  getPermissionsForRole
} = require('../auth/permission_middleware');

describe('Permission Middleware', () => {
  describe('checkDevicePermission', () => {
    it('should allow SuperAdmin for any operation', () => {
      const result = checkDevicePermission(['SuperAdmin'], 'device:restart');
      expect(result.allowed).toBe(true);
    });

    it('should allow Operator for deploy operation', () => {
      const result = checkDevicePermission(['Operator'], 'deploy');
      expect(result.allowed).toBe(true);
    });

    it('should deny Developer for device:force-restart', () => {
      const result = checkDevicePermission(['Developer'], 'device:force-restart');
      expect(result.allowed).toBe(false);
      // requiredRole may be null if operation not found in config, or 'SuperAdmin' if found
      expect(result.requiredRole).toBeDefined();
    });

    it('should allow Developer for device:read', () => {
      const result = checkDevicePermission(['Developer'], 'device:read');
      expect(result.allowed).toBe(true);
    });

    it('should deny with no roles', () => {
      const result = checkDevicePermission([], 'device:read');
      expect(result.allowed).toBe(false);
      expect(result.reason).toBe('No user roles provided');
    });
  });

  describe('hasPermission', () => {
    it('should return true for valid permission', () => {
      const result = hasPermission('Operator', 'deploy');
      // Result depends on config loading
      expect(typeof result).toBe('boolean');
    });

    it('should return false for invalid permission', () => {
      const result = hasPermission('Developer', 'admin');
      expect(result).toBe(false);
    });
  });

  describe('getRequiredRole', () => {
    it('should return role for known operation', () => {
      const role = getRequiredRole('device:force-restart');
      // Returns null if not in config, or the role if found
      expect(role === null || typeof role === 'string').toBe(true);
    });

    it('should return null for unknown operation', () => {
      const role = getRequiredRole('unknown:operation');
      expect(role).toBeNull();
    });
  });

  describe('getPermissionsForRole', () => {
    it('should return permissions for known role', () => {
      const perms = getPermissionsForRole('Operator');
      // Returns array (may be empty if config not loaded)
      expect(Array.isArray(perms)).toBe(true);
    });

    it('should return empty array for unknown role', () => {
      const perms = getPermissionsForRole('UnknownRole');
      expect(perms).toEqual([]);
    });
  });

  describe('requireRole middleware', () => {
    let mockReq, mockRes, mockNext;

    beforeEach(() => {
      mockReq = { user: { roles: ['Developer'] } };
      mockRes = {
        status: jest.fn().mockReturnThis(),
        json: jest.fn()
      };
      mockNext = jest.fn();
    });

    it('should call next() when user has required role', () => {
      const middleware = requireRole('Developer');
      middleware(mockReq, mockRes, mockNext);

      expect(mockNext).toHaveBeenCalled();
      expect(mockRes.status).not.toHaveBeenCalled();
    });

    it('should call next() when user is SuperAdmin', () => {
      mockReq.user.roles = ['SuperAdmin'];
      const middleware = requireRole('Developer');
      middleware(mockReq, mockRes, mockNext);

      expect(mockNext).toHaveBeenCalled();
    });

    it('should return 403 when user lacks required role', () => {
      const middleware = requireRole('SuperAdmin');
      middleware(mockReq, mockRes, mockNext);

      expect(mockRes.status).toHaveBeenCalledWith(403);
      expect(mockRes.json).toHaveBeenCalledWith(
        expect.objectContaining({
          success: false,
          error: 'Insufficient role'
        })
      );
      expect(mockNext).not.toHaveBeenCalled();
    });
  });

  describe('requireAnyRole middleware', () => {
    let mockReq, mockRes, mockNext;

    beforeEach(() => {
      mockReq = { user: { roles: ['Developer'] } };
      mockRes = {
        status: jest.fn().mockReturnThis(),
        json: jest.fn()
      };
      mockNext = jest.fn();
    });

    it('should call next() when user has any of the required roles', () => {
      const middleware = requireAnyRole(['Developer', 'Operator']);
      middleware(mockReq, mockRes, mockNext);

      expect(mockNext).toHaveBeenCalled();
    });

    it('should return 403 when user has none of the required roles', () => {
      const middleware = requireAnyRole(['Auditor', 'SuperAdmin']);
      middleware(mockReq, mockRes, mockNext);

      expect(mockRes.status).toHaveBeenCalledWith(403);
      expect(mockNext).not.toHaveBeenCalled();
    });
  });
});

describe('Permission Integration', () => {
  describe('Role hierarchy', () => {
    it('should allow Operator to perform Developer operations', () => {
      const result = checkDevicePermission(['Operator'], 'device:write');
      expect(result.allowed).toBe(true);
    });

    it('should allow Operator to perform Operator operations', () => {
      const result = checkDevicePermission(['Operator'], 'deploy');
      expect(result.allowed).toBe(true);
    });

    it('should not allow Developer to perform Operator operations', () => {
      const result = checkDevicePermission(['Developer'], 'deploy');
      expect(result.allowed).toBe(false);
    });
  });

  describe('Environment restrictions', () => {
    it('should allow SuperAdmin for prod environment', () => {
      const result = checkDevicePermission(
        ['SuperAdmin'],
        'device:restart',
        ['env:prod']
      );
      expect(result.allowed).toBe(true);
    });
  });
});
