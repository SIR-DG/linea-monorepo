// Code generated by bavard DO NOT EDIT

package wizard
import "strconv"

func (c *CoinField) WithTags(tags ...string) *CoinField {
	c.metadata.tags = append(c.metadata.tags, tags...)
	return c
}

func (c *CoinField) WithName(name string) *CoinField {
	c.metadata.name = name
	return c
}

func (c *CoinField) WithDoc(doc string) *CoinField {
	c.metadata.doc = doc
	return c
}

func (c *CoinField) Tags() []string {
	return c.metadata.tags
}

func (c *CoinField) ListTags() []string {
	return c.metadata.listTags()
}

func (c *CoinField) HasTag(tag string) bool {
	tags := c.Tags()
	for i := range tags {
		if tags[i] == tag {
			return true
		}
	}
	return false
}

func (c *CoinField) String() string {
	return c.metadata.scope.getFullScope() + "/" + c.metadata.nameOrDefault(c) + "/" + strconv.Itoa(int(c.metadata.id))
}

func (c *CoinField) Explain() string {
	return c.metadata.explain(c)
}
func (c *CoinField) id() id {
	return c.metadata.id
}
