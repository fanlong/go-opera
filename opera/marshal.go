package opera

import "encoding/json"

func UpdateRules(src Rules, diff []byte) (res Rules, err error) {
	changed := src
	err = json.Unmarshal(diff, &changed)
	if err != nil {
		return
	}
	// protect readonly fields
	res = changed
	res.NetworkID = src.NetworkID
	res.Name = src.Name
	return
}