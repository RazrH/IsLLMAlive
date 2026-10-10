package tray

import "sync"

var loadIcons = sync.OnceValues(func() (map[string][]byte, error) {
	icons := make(map[string][]byte, 4)
	for _, color := range []string{"#9E9E9E", "#69B72A", "#F0E442", "#D50000"} {
		data, err := GenerateIcon(color)
		if err != nil {
			return nil, err
		}
		icons[color] = data
	}
	return icons, nil
})

// cachedIcon returns immutable process-lifetime icon bytes for the four known colors.
func cachedIcon(color string) ([]byte, error) {
	icons, err := loadIcons()
	if err != nil {
		return nil, err
	}
	return icons[color], nil
}
