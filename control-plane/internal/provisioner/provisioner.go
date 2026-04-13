package provisioner

import (
	"context"

	"go.uber.org/zap"
)

// Provisioner defines the interface for provisioning cloud resources
type Provisioner interface {
	ProvisionNamespace(ctx context.Context, name string) error
	ProvisionDatabase(ctx context.Context, name string) error
}

type cloudProvisioner struct {
	logger *zap.Logger
}

// New returns a new Provisioner instance
func New(logger *zap.Logger) Provisioner {
	return &cloudProvisioner{
		logger: logger,
	}
}

func (p *cloudProvisioner) ProvisionNamespace(ctx context.Context, name string) error {
	p.logger.Info("provisioning namespace", zap.String("name", name))
	return nil
}

func (p *cloudProvisioner) ProvisionDatabase(ctx context.Context, name string) error {
	p.logger.Info("provisioning database", zap.String("name", name))
	return nil
}
