package drivers

import (
	_ "github.com/OpenListTeam/OpenList/v4/drivers/dropbox"
	_ "github.com/OpenListTeam/OpenList/v4/drivers/local"
	_ "github.com/OpenListTeam/OpenList/v4/drivers/s3"
)

// All do nothing,just for import
// same as _ import
func All() {
}
