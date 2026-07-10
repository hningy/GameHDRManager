//go:build windows && cgo

package hdr

/*
#cgo LDFLAGS: -luser32
#include <windows.h>
#include <stdlib.h>

// Returns 1 when an active display reports Advanced Color/HDR enabled,
// 0 when it reports disabled, and -1 when the API is unavailable.
static int ghdr_advanced_color_enabled(void) {
    UINT paths = 0, modes = 0;
    if (GetDisplayConfigBufferSizes(QDC_ONLY_ACTIVE_PATHS, &paths, &modes) != ERROR_SUCCESS || paths == 0) return -1;
    DISPLAYCONFIG_PATH_INFO *p = (DISPLAYCONFIG_PATH_INFO*)calloc(paths, sizeof(*p));
    DISPLAYCONFIG_MODE_INFO *m = (DISPLAYCONFIG_MODE_INFO*)calloc(modes ? modes : 1, sizeof(*m));
    if (!p || !m) { free(p); free(m); return -1; }
    LONG rc = QueryDisplayConfig(QDC_ONLY_ACTIVE_PATHS, &paths, p, &modes, m, NULL);
    if (rc != ERROR_SUCCESS) { free(p); free(m); return -1; }
    int enabled = 0, found = 0;
    for (UINT i = 0; i < paths; ++i) {
        DISPLAYCONFIG_GET_ADVANCED_COLOR_INFO info;
        ZeroMemory(&info, sizeof(info));
        info.header.type = DISPLAYCONFIG_DEVICE_INFO_GET_ADVANCED_COLOR_INFO;
        info.header.size = sizeof(info);
        info.header.adapterId = p[i].targetInfo.adapterId;
        info.header.id = p[i].targetInfo.id;
        if (DisplayConfigGetDeviceInfo(&info.header) == ERROR_SUCCESS) {
            found = 1;
            if (info.advancedColorEnabled) enabled = 1;
        }
    }
    free(p); free(m);
    return found ? enabled : -1;
}
*/
import "C"

func advancedColorState() (State, bool) {
	switch C.ghdr_advanced_color_enabled() {
	case 1:
		return On, true
	case 0:
		return Off, true
	default:
		return Unknown, false
	}
}
