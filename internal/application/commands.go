package application

import (
	"context"
	"strings"
	"time"

	"stage-rigging-clearance/internal/domain"
)

func (s *Service) PrepareBaseline(ctx context.Context, cmd CaseCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "prepare_baseline", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error { return c.PrepareBaseline(now) })
}

func (s *Service) AddAsset(ctx context.Context, cmd AddAssetCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "add_asset", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error { _, err := c.AddAsset(cmd.Asset, now); return err })
}

func (s *Service) AddAssetsBatch(ctx context.Context, cmd AddAssetsBatchCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	return s.mutateResult(ctx, cmd.CaseNumber, "add_assets_batch", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) (any, error) {
			assets, err := c.AddAssetsBatch(cmd.Assets, now)
			if err != nil {
				return nil, err
			}
			results := make([]batchAssetResult, len(assets))
			for index, asset := range assets {
				results[index] = batchAssetResult{ID: asset.ID, AssetCode: asset.AssetCode,
					NormalizedAssetCode: asset.AssetCode, Result: "registered"}
			}
			return batchAssetEnvelope{CaseNumber: c.CaseNumber, Results: results,
				AddedCount: len(results), LatestVersion: c.Version}, nil
		})
}

func (s *Service) LockBaseline(ctx context.Context, cmd CaseCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "lock_baseline", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error { return c.LockBaseline(now) })
}

func (s *Service) RecordTest(ctx context.Context, cmd RecordTestCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	cmd.Test.RecordedBy = cmd.Actor
	return s.mutate(ctx, cmd.CaseNumber, "record_test", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			_, _, err := c.RecordTest(cmd.Test, now)
			return err
		})
}

func (s *Service) AddObservedDefect(ctx context.Context, cmd AddDefectCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "add_observed_defect", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			_, err := c.AddObservedDefect(cmd.AssetID, cmd.Severity, cmd.Description, now)
			return err
		})
}

func (s *Service) RemediateDefect(ctx context.Context, cmd RemediateDefectCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleMaintenance); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "remediate_defect", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			_, err := c.RemediateDefectBy(cmd.DefectID, cmd.Evidence, cmd.Actor, now)
			return err
		})
}

func (s *Service) ReviewDefect(ctx context.Context, cmd ReviewDefectCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleReviewer); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "review_defect", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			if err := ensureIndependent(c, cmd.Actor); err != nil {
				return err
			}
			_, err := c.ReviewDefect(cmd.DefectID, cmd.Actor, cmd.Accepted, cmd.Comment, now)
			return err
		})
}

func (s *Service) SubmitReview(ctx context.Context, cmd CaseCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleInspector, RoleMaintenance); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "submit_review", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error { return c.SubmitForReview(now) })
}

func (s *Service) ReturnReview(ctx context.Context, cmd ReviewCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleReviewer); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "return_review", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			if err := ensureIndependent(c, cmd.Actor); err != nil {
				return err
			}
			return c.ReturnReview(cmd.Actor, cmd.Comment, now)
		})
}

func (s *Service) ApproveReview(ctx context.Context, cmd ReviewCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleReviewer); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "approve_review", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			if err := ensureIndependent(c, cmd.Actor); err != nil {
				return err
			}
			return c.ApproveReview(cmd.Actor, cmd.Comment, now)
		})
}

func (s *Service) FreezeReport(ctx context.Context, cmd CaseCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleReviewer); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "freeze_report", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			_, err := c.FreezeReport(cmd.Actor, now)
			return err
		})
}

func (s *Service) IssueCertificate(ctx context.Context, cmd CaseCommand) (*Result, error) {
	if err := validateMeta(cmd.CommandMeta, RoleReviewer); err != nil {
		return nil, err
	}
	return s.mutate(ctx, cmd.CaseNumber, "issue_certificate", cmd.CommandMeta, cmd,
		func(c *domain.InspectionCase, now time.Time) error {
			_, err := c.IssueCertificate(cmd.Actor, now)
			return err
		})
}

func ensureIndependent(c *domain.InspectionCase, reviewer string) error {
	reviewer = strings.TrimSpace(reviewer)
	for _, record := range c.Tests {
		if strings.EqualFold(strings.TrimSpace(record.RecordedBy), reviewer) {
			return domain.NewRuleError(domain.CodeForbidden, "独立复核员不能是测试采集责任人")
		}
	}
	return nil
}
