package domain

import (
	"cmp"
	"slices"
)

// Feature names what a metered call was for. It is stored as a slug, so a
// release that adds a feature adds no migration.
type Feature string

// The features of the AI assistant, one for each thing a teacher can ask for.
const (
	FeatureGenerate    Feature = "generate"
	FeatureCheck       Feature = "check"
	FeatureVariant     Feature = "variant"
	FeatureExplanation Feature = "explanation"
	FeatureDistractors Feature = "distractors"
	FeatureScore       Feature = "score"
	FeatureInsights    Feature = "insights"
	FeatureCleanup     Feature = "cleanup"
	FeatureImport      Feature = "import"
)

// Category is a group of features the Settings card sums together.
type Category string

// The five rows of the usage card, in the order it draws them.
const (
	CategoryGeneration   Category = "generation"
	CategoryGrading      Category = "grading"
	CategoryExplanations Category = "explanations"
	CategoryImports      Category = "imports"
	CategoryChecks       Category = "checks"
)

type featureInfo struct {
	category Category
	ceiling  int64
}

var features = map[Feature]featureInfo{
	FeatureGenerate:    {CategoryGeneration, 200},
	FeatureVariant:     {CategoryGeneration, 300},
	FeatureScore:       {CategoryGrading, 30},
	FeatureExplanation: {CategoryExplanations, 20},
	FeatureDistractors: {CategoryExplanations, 20},
	FeatureImport:      {CategoryImports, 60},
	FeatureCheck:       {CategoryChecks, 300},
	FeatureInsights:    {CategoryChecks, 60},
	FeatureCleanup:     {CategoryChecks, 200},
}

var categories = []Category{CategoryGeneration, CategoryGrading, CategoryExplanations, CategoryImports, CategoryChecks}

// Known reports whether f is a feature this binary meters.
func (f Feature) Known() bool {
	_, ok := features[f]
	return ok
}

// Category is the usage card row that sums f, and the empty Category for a
// feature this binary does not know.
func (f Feature) Category() Category {
	return features[f].category
}

// Ceiling is the most credits one call of f reserves, which the plan sets from
// the feature's tier, its largest input and its token limit (the import reads
// a page at a time, so its ceiling is for one page). It is zero for an unknown
// feature.
func (f Feature) Ceiling() int64 {
	return features[f].ceiling
}

// Categories returns the usage card's rows in the order it draws them.
func Categories() []Category {
	return slices.Clone(categories)
}

// Features returns every feature in the order of Categories, then by name.
func Features() []Feature {
	out := make([]Feature, 0, len(features))
	for f := range features {
		out = append(out, f)
	}
	slices.SortFunc(out, func(a, b Feature) int {
		if ca, cb := slices.Index(categories, a.Category()), slices.Index(categories, b.Category()); ca != cb {
			return ca - cb
		}
		return cmp.Compare(a, b)
	})
	return out
}
