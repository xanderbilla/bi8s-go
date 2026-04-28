package app

import (
	"context"

	awsinfra "github.com/xanderbilla/bi8s-go/internal/aws"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

type HealthCheck func(ctx context.Context) error

type Application struct {
	Config           Config
	Clients          *awsinfra.Clients
	MovieService     *service.MovieService
	PersonService    *service.PersonService
	AttributeService *service.AttributeService
	EncoderService   *service.EncoderService
	HealthChecks     map[string]HealthCheck
}
