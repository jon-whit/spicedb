package tuple

import (
	"fmt"
	"regexp"

	v0 "github.com/authzed/authzed-go/proto/authzed/api/v0"
	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/jzelinskie/stringz"
)

const (
	ellipsis = "..."
)

const (
	namespaceNameExpr = "([a-z][a-z0-9_]{2,61}[a-z0-9]/)?[a-z][a-z0-9_]{2,62}[a-z0-9]"
	objectIDExpr      = "[a-zA-Z0-9_][a-zA-Z0-9/_-]{0,127}"
	relationExpr      = "[a-z][a-z0-9_]{2,62}[a-z0-9]"
)

var onrExpr = fmt.Sprintf(
	`(?P<resourceType>(%s)):(?P<resourceID>%s)#(?P<resourceRel>%s)`,
	namespaceNameExpr,
	objectIDExpr,
	relationExpr,
)

var subjectExpr = fmt.Sprintf(
	`(?P<subjectType>(%s)):(?P<subjectID>%s)(#(?P<subjectRel>%s|\.\.\.))?`,
	namespaceNameExpr,
	objectIDExpr,
	relationExpr,
)

var (
	onrRegex     = regexp.MustCompile(fmt.Sprintf("^%s$", onrExpr))
	subjectRegex = regexp.MustCompile(fmt.Sprintf("^%s$", subjectExpr))
)

var parserRegex = regexp.MustCompile(
	fmt.Sprintf(
		`^%s@%s$`,
		onrExpr,
		subjectExpr,
	),
)

// String converts a tuple to a string.
func String(tpl *v0.RelationTuple) string {
	if tpl == nil || tpl.ObjectAndRelation == nil || tpl.User == nil || tpl.User.GetUserset() == nil {
		return ""
	}

	return fmt.Sprintf("%s@%s", StringONR(tpl.ObjectAndRelation), StringONR(tpl.User.GetUserset()))
}

// RelString converts a relationship into a string.
func RelString(tpl *v1.Relationship) string {
	return String(FromRelationship(tpl))
}

// Parse converts a serialized tuple into the proto version
func Parse(tpl string) *v0.RelationTuple {
	groups := parserRegex.FindStringSubmatch(tpl)
	if len(groups) == 0 {
		return nil
	}

	subjectRelation := ellipsis
	subjectRelIndex := stringz.SliceIndex(parserRegex.SubexpNames(), "subjectRel")
	if len(groups[subjectRelIndex]) > 0 {
		subjectRelation = groups[subjectRelIndex]
	}

	return &v0.RelationTuple{
		ObjectAndRelation: &v0.ObjectAndRelation{
			Namespace: groups[stringz.SliceIndex(parserRegex.SubexpNames(), "resourceType")],
			ObjectId:  groups[stringz.SliceIndex(parserRegex.SubexpNames(), "resourceID")],
			Relation:  groups[stringz.SliceIndex(parserRegex.SubexpNames(), "resourceRel")],
		},
		User: &v0.User{UserOneof: &v0.User_Userset{Userset: &v0.ObjectAndRelation{
			Namespace: groups[stringz.SliceIndex(parserRegex.SubexpNames(), "subjectType")],
			ObjectId:  groups[stringz.SliceIndex(parserRegex.SubexpNames(), "subjectID")],
			Relation:  subjectRelation,
		}}},
	}
}

func Create(tpl *v0.RelationTuple) *v0.RelationTupleUpdate {
	return &v0.RelationTupleUpdate{
		Operation: v0.RelationTupleUpdate_CREATE,
		Tuple:     tpl,
	}
}

func Touch(tpl *v0.RelationTuple) *v0.RelationTupleUpdate {
	return &v0.RelationTupleUpdate{
		Operation: v0.RelationTupleUpdate_TOUCH,
		Tuple:     tpl,
	}
}

func Delete(tpl *v0.RelationTuple) *v0.RelationTupleUpdate {
	return &v0.RelationTupleUpdate{
		Operation: v0.RelationTupleUpdate_DELETE,
		Tuple:     tpl,
	}
}

// ToRelationship converts a RelationTuple into a Relationship.
func ToRelationship(tpl *v0.RelationTuple) *v1.Relationship {
	if err := tpl.Validate(); err != nil {
		panic(fmt.Sprintf("invalid tuple: %s %s", String(tpl), err))
	}

	return &v1.Relationship{
		Resource: &v1.ObjectReference{
			ObjectType: tpl.ObjectAndRelation.Namespace,
			ObjectId:   tpl.ObjectAndRelation.ObjectId,
		},
		Relation: tpl.ObjectAndRelation.Relation,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{
				ObjectType: tpl.User.GetUserset().Namespace,
				ObjectId:   tpl.User.GetUserset().ObjectId,
			},
			OptionalRelation: stringz.Default(tpl.User.GetUserset().Relation, "", "..."),
		},
	}
}

// ToFilter converts a RelationTuple into a RelationshipFilter.
func ToFilter(tpl *v0.RelationTuple) *v1.RelationshipFilter {
	if err := tpl.Validate(); err != nil {
		panic(fmt.Sprintf("invalid tuple: %s %s", String(tpl), err))
	}

	return &v1.RelationshipFilter{
		ResourceType:       tpl.ObjectAndRelation.Namespace,
		OptionalResourceId: tpl.ObjectAndRelation.ObjectId,
		OptionalRelation:   tpl.ObjectAndRelation.Relation,
		OptionalSubjectFilter: &v1.SubjectFilter{
			SubjectType:       tpl.User.GetUserset().Namespace,
			OptionalSubjectId: tpl.User.GetUserset().ObjectId,
			OptionalRelation: &v1.SubjectFilter_RelationFilter{
				Relation: stringz.Default(tpl.User.GetUserset().Relation, "", "..."),
			},
		},
	}
}

// UpdateToRelationshipUpdate converts a RelationTupleUpdate into a
// RelationshipUpdate.
func UpdateToRelationshipUpdate(update *v0.RelationTupleUpdate) *v1.RelationshipUpdate {
	var op v1.RelationshipUpdate_Operation
	switch update.Operation {
	case v0.RelationTupleUpdate_CREATE:
		op = v1.RelationshipUpdate_OPERATION_CREATE
	case v0.RelationTupleUpdate_DELETE:
		op = v1.RelationshipUpdate_OPERATION_DELETE
	case v0.RelationTupleUpdate_TOUCH:
		op = v1.RelationshipUpdate_OPERATION_TOUCH
	default:
		panic("unknown tuple mutation")
	}

	return &v1.RelationshipUpdate{
		Operation:    op,
		Relationship: ToRelationship(update.Tuple),
	}
}

// FromRelationship converts a Relationship into a RelationTuple.
func FromRelationship(r *v1.Relationship) *v0.RelationTuple {
	if err := r.Validate(); err != nil {
		panic(fmt.Sprintf("invalid relationship: %s %s", RelString(r), err))
	}

	return &v0.RelationTuple{
		ObjectAndRelation: &v0.ObjectAndRelation{
			Namespace: r.Resource.ObjectType,
			ObjectId:  r.Resource.ObjectId,
			Relation:  r.Relation,
		},
		User: &v0.User{UserOneof: &v0.User_Userset{Userset: &v0.ObjectAndRelation{
			Namespace: r.Subject.Object.ObjectType,
			ObjectId:  r.Subject.Object.ObjectId,
			Relation:  stringz.DefaultEmpty(r.Subject.OptionalRelation, "..."),
		}}},
	}
}

// UpdateFromRelationshipUpdate converts a RelationshipUpdate into a
// RelationTupleUpdate.
func UpdateFromRelationshipUpdate(update *v1.RelationshipUpdate) *v0.RelationTupleUpdate {
	var op v0.RelationTupleUpdate_Operation
	switch update.Operation {
	case v1.RelationshipUpdate_OPERATION_CREATE:
		op = v0.RelationTupleUpdate_CREATE
	case v1.RelationshipUpdate_OPERATION_DELETE:
		op = v0.RelationTupleUpdate_DELETE
	case v1.RelationshipUpdate_OPERATION_TOUCH:
		op = v0.RelationTupleUpdate_TOUCH
	default:
		panic("unknown tuple mutation")
	}

	return &v0.RelationTupleUpdate{
		Operation: op,
		Tuple:     FromRelationship(update.Relationship),
	}
}
