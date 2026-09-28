package tui

import (
	"testing"

	"github.com/albersg/dotfiles/installer/internal/system"
)

// Byte quantities the table is written in, so every host size reads as the
// number the arithmetic in the case comment uses.
const (
	testMiB = 1 << 20
	testGiB = 1 << 30
)

// TestPlanWSLResources pins the WSL proportional policy. Every case states its
// own arithmetic, because the numbers are the contract: memory is half the host
// clamped to (host - 2 GiB) for Windows, rounded down to 512 MiB and omitted
// below 1 GiB; processors are all logical CPUs; swap is a quarter of the planned
// memory, rounded down to 512 MiB and omitted when that is zero.
func TestPlanWSLResources(t *testing.T) {
	tests := []struct {
		name string
		host system.HostResources
		want WSLResources
	}{
		{
			name: "4 GiB, 4 CPUs: min(2 GiB, 4-2=2 GiB) = 2048 MiB; swap = 2048/4 = 512",
			host: system.HostResources{MemoryBytes: 4 * testGiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 2048, Processors: 4, SwapMB: 512},
		},
		{
			name: "6 GiB, 8 CPUs: min(3 GiB, 6-2=4 GiB) = 3072 MiB; swap = 768 -> 512",
			host: system.HostResources{MemoryBytes: 6 * testGiB, LogicalCPUs: 8},
			want: WSLResources{MemoryMB: 3072, Processors: 8, SwapMB: 512},
		},
		{
			// Half = 8388608000 B = 8000 MiB. The reserve leaves
			// 14629752352 B (about 13952 MiB), which is larger, so the half
			// wins. 8000 = 15x512 + 320 -> 15x512 = 7680. Swap = 7680/4 = 1920
			// = 3x512 + 384 -> 3x512 = 1536.
			name: "16777216000 B, 12 CPUs: min(8000 MiB, 13952 MiB) = 8000 -> 7680; swap = 1920 -> 1536",
			host: system.HostResources{MemoryBytes: 16777216000, LogicalCPUs: 12},
			want: WSLResources{MemoryMB: 7680, Processors: 12, SwapMB: 1536},
		},
		{
			name: "32 GiB, 16 CPUs: min(16 GiB, 30 GiB) = 16384 MiB; swap = 16384/4 = 4096",
			host: system.HostResources{MemoryBytes: 32 * testGiB, LogicalCPUs: 16},
			want: WSLResources{MemoryMB: 16384, Processors: 16, SwapMB: 4096},
		},
		{
			name: "64 GiB, 32 CPUs: min(32 GiB, 62 GiB) = 32768 MiB; swap = 32768/4 = 8192",
			host: system.HostResources{MemoryBytes: 64 * testGiB, LogicalCPUs: 32},
			want: WSLResources{MemoryMB: 32768, Processors: 32, SwapMB: 8192},
		},
		{
			// The reserve clamp is what decides here: half is 1536 MiB, but
			// host - 2 GiB is only 1024 MiB, so 1024 is the target. Swap would
			// be 1024/4 = 256 MiB, which rounds down to nothing.
			name: "3 GiB, 4 CPUs: min(1536 MiB, 3-2=1 GiB) = 1024 MiB; swap = 256 -> 0 (omitted)",
			host: system.HostResources{MemoryBytes: 3 * testGiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 1024, Processors: 4, SwapMB: 0},
		},
		{
			// The reserve boundary itself: `memoryBytes <= reserve` is the
			// no-room branch, so a host at exactly 2 GiB omits memory instead of
			// planning a 0 MiB or wrapped value.
			name: "2 GiB, 4 CPUs: host == 2 GiB reserve is the no-room boundary; memory and swap omitted",
			host: system.HostResources{MemoryBytes: 2 * testGiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 0, Processors: 4, SwapMB: 0},
		},
		{
			// The gap between the reserve boundary above and the 3 GiB case
			// below: half is 1280 MiB, but the reserve of 2 GiB leaves only
			// 2560-2048 = 512 MiB, so the clamp wins and the target lands under
			// the 1 GiB floor. Memory and swap are omitted, processors are not.
			// Without this case the floor was dead code: 2 GiB returns through
			// `<= reserve` and 3 GiB sits exactly on the floor, so deleting the
			// check still left the whole table green while a 2.5 GiB host was
			// rendered memory=512.
			name: "2.5 GiB, 4 CPUs: min(1280 MiB, 2560-2048=512 MiB) = 512 MiB, below the 1 GiB floor, so memory and swap are omitted",
			host: system.HostResources{MemoryBytes: 2560 * testMiB, LogicalCPUs: 4},
			want: WSLResources{MemoryMB: 0, Processors: 4, SwapMB: 0},
		},
		{
			// A host at or below the reserve has no room to give: the unsigned
			// subtraction must not wrap into a huge "available" value, and the
			// target is below the 1 GiB floor anyway.
			name: "1 GiB, 2 CPUs: host <= 2 GiB reserve, so memory and swap are omitted; processors = 2",
			host: system.HostResources{MemoryBytes: 1 * testGiB, LogicalCPUs: 2},
			want: WSLResources{MemoryMB: 0, Processors: 2, SwapMB: 0},
		},
		{
			// 5632 MiB: half is 2816 MiB, the reserve leaves 3584 MiB, so the
			// half wins. 2816 = 5x512 + 256 -> 5x512 = 2560. Swap = 2560/4 =
			// 640 = 1x512 + 128 -> 512.
			name: "5.5 GiB, 8 CPUs: min(2816 MiB, 5632-2048=3584 MiB) = 2816 -> 2560; swap = 640 -> 512",
			host: system.HostResources{MemoryBytes: 5632 * testMiB, LogicalCPUs: 8},
			want: WSLResources{MemoryMB: 2560, Processors: 8, SwapMB: 512},
		},
		{
			name: "8 GiB, unknown CPUs: min(4 GiB, 6 GiB) = 4096 MiB; swap = 1024; processors omitted",
			host: system.HostResources{MemoryBytes: 8 * testGiB, LogicalCPUs: 0},
			want: WSLResources{MemoryMB: 4096, Processors: 0, SwapMB: 1024},
		},
		{
			name: "unknown host: all three keys omitted, so WSL applies its own defaults",
			host: system.HostResources{},
			want: WSLResources{MemoryMB: 0, Processors: 0, SwapMB: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PlanWSLResources(tt.host)

			if got != tt.want {
				t.Errorf("PlanWSLResources(%+v) = %+v, want %+v", tt.host, got, tt.want)
			}
		})
	}
}
