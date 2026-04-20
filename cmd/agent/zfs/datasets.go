package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// ListDatasets returns all ZFS filesystems and volumes on the host.
// Snapshots and bookmarks are intentionally excluded — the server schema
// only models filesystems/volumes.
func ListDatasets() ([]Dataset, error) {
	zfsPath := findZfsCommand()
	if zfsPath == "" {
		return nil, fmt.Errorf("zfs command not found")
	}

	// -H: no header, tab-separated
	// -p: exact (parsable) byte counts
	// -t filesystem,volume: skip snapshots/bookmarks
	// Columns must match parseDatasetList below.
	cmd := exec.Command(zfsPath, "list", "-H", "-p",
		"-t", "filesystem,volume",
		"-o", "name,used,available,referenced,mountpoint,compressratio,quota")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "no datasets available") {
			return []Dataset{}, nil
		}
		return nil, fmt.Errorf("zfs list failed: %v - %s", err, stderr.String())
	}

	return parseDatasetList(stdout.String()), nil
}

func parseDatasetList(output string) []Dataset {
	var datasets []Dataset
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		name := fields[0]
		ds := Dataset{
			Name:            name,
			PoolName:        poolNameFromDataset(name),
			UsedBytes:       parseBytes(fields[1]),
			AvailableBytes:  parseBytes(fields[2]),
			ReferencedBytes: parseBytes(fields[3]),
			Mountpoint:      normalizeMountpoint(fields[4]),
			// `zfs list -p` emits compressratio as a decimal without the
			// trailing "x", but trim it defensively in case the output
			// format varies across ZFS builds.
			CompressRatio: parseFloat(strings.TrimSuffix(fields[5], "x")),
			QuotaBytes:    parseBytes(fields[6]),
		}

		datasets = append(datasets, ds)
	}

	return datasets
}

// poolNameFromDataset extracts the pool name from a dataset path.
// "Tank"         → "Tank"
// "Tank/media"   → "Tank"
// "Tank/media/a" → "Tank"
func poolNameFromDataset(name string) string {
	if i := strings.IndexByte(name, '/'); i > 0 {
		return name[:i]
	}
	return name
}

// normalizeMountpoint maps the sentinels zfs prints for unmounted datasets
// ("-", "none", "legacy") to an empty string so the server stores NULL-ish
// values consistently.
func normalizeMountpoint(mp string) string {
	mp = strings.TrimSpace(mp)
	switch mp {
	case "-", "none", "legacy":
		return ""
	}
	return mp
}
