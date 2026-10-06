package object

import "fmt"

// This work was started so that an object or mob from one zone could be referred to
// in the definition files of a different zone. But that's a bigger amount of work
// than we're ready for; alternatively it might be a bad idea and need some re-thinking.

type Id struct {
	DefinitionId string
	ZoneId       string
}

func NewObjectId(id string, zoneId string) Id {
	return Id{
		DefinitionId: id,
		ZoneId:       zoneId,
	}
}

func (id *Id) String() string {
	return fmt.Sprintf("%s:%s", id.ZoneId, id.DefinitionId)
}

// Ref is the id the way content names an object: "zone/id".
func (id Id) Ref() string {
	return id.ZoneId + "/" + id.DefinitionId
}
