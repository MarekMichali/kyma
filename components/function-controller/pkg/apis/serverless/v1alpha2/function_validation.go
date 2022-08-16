package v1alpha2

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/apimachinery/pkg/api/validation"
	v1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const ValidationConfigKey = "validation-config"

type MinFunctionReplicasValues struct {
	MinValue int32 `envconfig:"default=1"`
}

type MinFunctionResourcesValues struct {
	MinRequestCPU    string `envconfig:"default=10m"`
	MinRequestMemory string `envconfig:"default=16Mi"`
}

type MinBuildJobResourcesValues struct {
	MinRequestCPU    string `envconfig:"default=200m"`
	MinRequestMemory string `envconfig:"default=200Mi"`
}

type MinFunctionValues struct {
	Replicas  MinFunctionReplicasValues
	Resources MinFunctionResourcesValues
}

type MinBuildJobValues struct {
	Resources MinBuildJobResourcesValues
}

type ValidationConfig struct {
	ReservedEnvs []string `envconfig:"default={}"`
	Function     MinFunctionValues
	BuildJob     MinBuildJobValues
}

type validationFunction func(*ValidationConfig) error

func (fn *Function) getBasicValidations() []validationFunction {
	return []validationFunction{
		fn.validateObjectMeta,
		fn.Spec.validateEnv,
		fn.Spec.validateLabels,
		fn.Spec.validateReplicas,
		fn.Spec.validateFunctionResources,
		fn.Spec.validateBuildResources,
		fn.Spec.validateSources,
	}
}

var (
	ErrUnknownFunctionType = fmt.Errorf("unknown function source type")
)

func (fn *Function) Validate(vc *ValidationConfig) error {
	validations := fn.getBasicValidations()

	switch {
	case fn.TypeOf(FunctionTypeInline):
		validations = append(validations, fn.Spec.validateInlineSrc, fn.Spec.validateInlineDeps)
		return runValidations(vc, validations...)

	case fn.TypeOf(FunctionTypeGit):
		gitAuthValidators := fn.Spec.gitAuthValidations()
		validations = append(validations, gitAuthValidators...)
		return runValidations(vc, validations...)

	default:
		validations = append(validations, unknownFunctionTypeValidator)
		return runValidations(vc, validations...)
	}
}

func unknownFunctionTypeValidator(_ *ValidationConfig) error {
	return ErrUnknownFunctionType
}

func runValidations(vc *ValidationConfig, vFuns ...validationFunction) error {
	allErrs := []string{}
	for _, vFun := range vFuns {
		if err := vFun(vc); err != nil {
			allErrs = append(allErrs, err.Error())
		}
	}
	return returnAllErrs("", allErrs)
}

func (fn *Function) validateObjectMeta(_ *ValidationConfig) error {
	nameFn := validation.ValidateNameFunc(validation.NameIsDNS1035Label)
	fieldPath := field.NewPath("metadata")
	if errs := validation.ValidateObjectMeta(&fn.ObjectMeta, true, nameFn, fieldPath); errs != nil {
		return errs.ToAggregate()
	}
	return nil
}

func (spec *FunctionSpec) validateGitRepoURL(_ *ValidationConfig) error {
	if urlIsSSH(spec.Source.GitRepository.URL) {
		return nil
	} else if _, err := url.ParseRequestURI(spec.Source.GitRepository.URL); err != nil {
		return fmt.Errorf("invalid source.gitRepository.URL value: %v", err)
	}
	return nil
}

func (spec *FunctionSpec) validateInlineSrc(_ *ValidationConfig) error {
	if spec.Source.Inline.Source == "" {
		return fmt.Errorf("empty source.inline.source value")
	}
	return nil
}

func (spec *FunctionSpec) validateInlineDeps(_ *ValidationConfig) error {
	if err := ValidateDependencies(spec.Runtime, spec.Source.Inline.Dependencies); err != nil {
		return errors.Wrap(err, "invalid source.inline.dependencies value")
	}
	return nil
}

func (spec *FunctionSpec) gitAuthValidations() []validationFunction {
	if spec.Source.GitRepository.Auth == nil {
		return []validationFunction{
			spec.validateRepository,
		}
	}
	return []validationFunction{
		spec.validateRepository,
		spec.validateGitAuthType,
		spec.validateGitAuthSecretName,
		spec.validateGitRepoURL,
	}
}

func (spec *FunctionSpec) validateGitAuthSecretName(_ *ValidationConfig) error {
	if strings.TrimSpace(spec.Source.GitRepository.Auth.SecretName) == "" {
		return errors.New("spec.source.gitRepository.auth.secretName is required")
	}
	return nil
}

var ErrInvalidGitRepositoryAuthType = fmt.Errorf("invalid git reposiotry authentication type")

func (spec *FunctionSpec) validateGitAuthType(_ *ValidationConfig) error {
	switch spec.Source.GitRepository.Auth.Type {
	case RepositoryAuthBasic, RepositoryAuthSSHKey:
		return nil
	default:
		return ErrInvalidGitRepositoryAuthType
	}
}

func (spec *FunctionSpec) validateEnv(vc *ValidationConfig) error {
	var allErrs []string
	envs := spec.Env
	reservedEnvs := vc.ReservedEnvs
	for _, env := range envs {
		errs := utilvalidation.IsEnvVarName(env.Name)
		for _, reservedEnv := range reservedEnvs {
			if env.Name == reservedEnv {
				errs = append(errs, "env name is reserved for the serverless domain")
			}
		}
		if len(errs) > 0 {
			allErrs = append(allErrs,
				errs...,
			)
		}
	}
	return returnAllErrs("invalid spec.env keys/values", allErrs)
}

func (spec *FunctionSpec) validateFunctionResources(vc *ValidationConfig) error {
	minMemory := resource.MustParse(vc.Function.Resources.MinRequestMemory)
	minCPU := resource.MustParse(vc.Function.Resources.MinRequestCPU)

	return validateResources(spec.ResourceConfiguration.Function.Resources, minMemory, minCPU, "spec.resourceConfiguration.function.resources")
}

func (spec *FunctionSpec) validateBuildResources(vc *ValidationConfig) error {
	minMemory := resource.MustParse(vc.BuildJob.Resources.MinRequestMemory)
	minCPU := resource.MustParse(vc.BuildJob.Resources.MinRequestCPU)

	return validateResources(spec.ResourceConfiguration.Build.Resources, minMemory, minCPU, "spec.resourceConfiguration.build.resources")
}

func (spec *FunctionSpec) validateSources(vc *ValidationConfig) error {
	sources := 0
	if spec.Source.GitRepository != nil {
		sources++
	}

	if spec.Source.Inline != nil {
		sources++
	}
	if sources == 1 {
		return nil
	}
	return errors.Errorf("spec.source should contains only 1 configuration of function")
}

func validateResources(resources corev1.ResourceRequirements, minMemory, minCPU resource.Quantity, parent string) error {
	limits := resources.Limits
	requests := resources.Requests
	allErrs := []string{}

	if requests != nil {
		allErrs = append(allErrs, validateRequests(resources, minMemory, minCPU, parent)...)
	}

	if limits != nil {
		allErrs = append(allErrs, validateLimites(resources, minMemory, minCPU, parent)...)
	}
	return returnAllErrs("invalid function resources", allErrs)
}

func validateRequests(resources corev1.ResourceRequirements, minMemory, minCPU resource.Quantity, parent string) []string {
	limits := resources.Limits
	requests := resources.Requests
	allErrs := []string{}

	if requests.Cpu().Cmp(minCPU) == -1 {
		allErrs = append(allErrs, fmt.Sprintf("%s.requests.cpu(%s) should be higher than minimal value (%s)",
			parent, requests.Cpu().String(), minCPU.String()))
	}
	if requests.Memory().Cmp(minMemory) == -1 {
		allErrs = append(allErrs, fmt.Sprintf("%s.requests.memory(%s) should be higher than minimal value (%s)",
			parent, requests.Memory().String(), minMemory.String()))
	}

	if limits == nil {
		return allErrs
	}

	if requests.Cpu().Cmp(*limits.Cpu()) == 1 {
		allErrs = append(allErrs, fmt.Sprintf("%s.limits.cpu(%s) should be higher than %s.requests.cpu(%s)",
			parent, limits.Cpu().String(), parent, requests.Cpu().String()))
	}
	if requests.Memory().Cmp(*limits.Memory()) == 1 {
		allErrs = append(allErrs, fmt.Sprintf("%s.limits.memory(%s) should be higher than %s.requests.memory(%s)",
			parent, limits.Memory().String(), parent, requests.Memory().String()))
	}

	return allErrs
}

func validateLimites(resources corev1.ResourceRequirements, minMemory, minCPU resource.Quantity, parent string) []string {
	limits := resources.Limits
	allErrs := []string{}

	if limits != nil {
		if limits.Cpu().Cmp(minCPU) == -1 {
			allErrs = append(allErrs, fmt.Sprintf("%s.limits.cpu(%s) should be higher than minimal value (%s)",
				parent, limits.Cpu().String(), minCPU.String()))
		}
		if limits.Memory().Cmp(minMemory) == -1 {
			allErrs = append(allErrs, fmt.Sprintf("%s.limits.memory(%s) should be higher than minimal value (%s)",
				parent, limits.Memory().String(), minMemory.String()))
		}
	}
	return allErrs
}

func (spec *FunctionSpec) validateReplicas(vc *ValidationConfig) error {
	if spec.ScaleConfig == nil {
		return errors.New("spec.scaleConfig can't be nil")
	}

	minValue := vc.Function.Replicas.MinValue
	maxReplicas := spec.ScaleConfig.MaxReplicas
	minReplicas := spec.ScaleConfig.MinReplicas
	allErrs := []string{}
	if maxReplicas != nil && minReplicas != nil && *minReplicas > *maxReplicas {
		allErrs = append(allErrs, fmt.Sprintf("spec.maxReplicas(%d) is less than spec.minReplicas(%d)",
			*maxReplicas, *minReplicas))
	}
	if minReplicas != nil && *minReplicas < minValue {
		allErrs = append(allErrs, fmt.Sprintf("spec.minReplicas(%d) is less than the smallest allowed value(%d)",
			*minReplicas, minValue))
	}
	if maxReplicas != nil && *maxReplicas < minValue {
		allErrs = append(allErrs, fmt.Sprintf("spec.maxReplicas(%d) is less than the smallest allowed value(%d)",
			*maxReplicas, minValue))
	}
	return returnAllErrs("invalid values", allErrs)
}

func (spec *FunctionSpec) validateLabels(_ *ValidationConfig) error {
	labels := spec.Template.Labels
	fieldPath := field.NewPath("spec.labels")

	errs := v1validation.ValidateLabels(labels, fieldPath)
	return errs.ToAggregate()
}

type property struct {
	name  string
	value string
}

func (spec *FunctionSpec) validateRepository(_ *ValidationConfig) error {
	return validateIfMissingFields([]property{
		{name: "spec.source.gitRepository.baseDir", value: spec.Source.GitRepository.BaseDir},
		{name: "spec.source.gitRepository.reference", value: spec.Source.GitRepository.Reference},
	}...)
}

func urlIsSSH(repoURL string) bool {
	exp, err := regexp.Compile(`((git|ssh?)|(git@[\w\.]+))(:(//)?)([\w\.@\:/\-~]+)(\.git)(/)?`)
	if err != nil {
		panic(err)
	}

	return exp.MatchString(repoURL)
}
