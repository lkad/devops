## 1. Backend - Weight API Enhancement

- [x] 1.1 Add UpdateResourceWeight API: PATCH /api/org/projects/:id/resources/:type/:resourceId
- [x] 1.2 Add GetResourceWeights API: GET /api/org/resources/:type/:resourceId/weights
- [x] 1.3 Validate weight value (0.0-1.0) in LinkResource and UpdateResourceWeight
- [x] 1.4 Update ListProjectResources to include weight in response

## 2. Backend - FinOps Weighted Report

- [x] 2.1 Modify FinOps export to respect resource weight when calculating costs
- [x] 2.2 Add unit test for weighted cost calculation
- [x] 2.3 Update FinOps report to show weight-adjusted counts

## 3. Frontend - Project Resources with Weights

- [x] 3.1 Update ProjectDetail.tsx resources tab to show weight column
- [ ] 3.2 Add inline weight editor (input field or slider)
- [x] 3.3 Show total weight sum and warn if > 100% or < 100%
- [ ] 3.4 Persist weight changes via PATCH API

## 4. Frontend - Host Detail Projects with Weights

- [x] 4.1 Update HostDetail.tsx 关联项目 tab to show weight column
- [ ] 4.2 Add inline weight editor
- [ ] 4.3 Show total weight sum and warn if > 100% or < 100%

## 5. Database Migration

- [x] 5.1 Set default weight=1.0 for existing project_resources records
- [x] 5.2 Add database migration for weight column if not exists

## 6. Integration Testing

- [x] 6.1 Test: Link resource with weight
- [x] 6.2 Test: Update existing resource weight
- [x] 6.3 Test: View resource with multiple project weights
- [x] 6.4 Test: FinOps report reflects weights