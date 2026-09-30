package rules

import (
	"errors"
)

type ObjectCategory string

const (
	ObjectCategoryNone     ObjectCategory = ""
	ObjectCategoryWeapon   ObjectCategory = "weapon"
	ObjectCategoryWand     ObjectCategory = "wand"
	ObjectCategoryStaff    ObjectCategory = "staff"
	ObjectCategoryTreasure ObjectCategory = "treasure"
	ObjectCategoryArmor    ObjectCategory = "armor"
	ObjectCategoryFood     ObjectCategory = "food"
	ObjectCategoryOther    ObjectCategory = "other"
	ObjectCategoryCorpse   ObjectCategory = "corpse"
)

var ErrUnknownObjectCategory = errors.New("unknown object category")

var objectCategories = enum[ObjectCategory]{
	"object categories",
	ErrUnknownObjectCategory,
	[]ObjectCategory{
		ObjectCategoryNone,
		ObjectCategoryWeapon,
		ObjectCategoryWand,
		ObjectCategoryStaff,
		ObjectCategoryTreasure,
		ObjectCategoryArmor,
		ObjectCategoryFood,
		ObjectCategoryOther,
		ObjectCategoryCorpse,
	},
}

func (c *ObjectCategory) UnmarshalText(b []byte) error {
	return objectCategories.unmarshal(c, b)
}

func ParseObjectCategory(s string) (ObjectCategory, error) {
	return objectCategories.parse(s)
}
