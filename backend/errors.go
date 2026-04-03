package main

import "errors"

// Sentinel error values for quota and structural validation failures.
//
// These values are returned by library methods and matched by the HTTP layer
// and WebDAV handler.  Keeping them in one place makes grep-based audits and
// future i18n of error messages straightforward.
//
// The string representation (err.Error()) is sent verbatim to the frontend
// as a JSON "error" field, so the values must not change without a matching
// update to the frontend error-handling logic.
var (
	// ErrLimitNoteSize is returned when a single note exceeds MaxNoteSize.
	ErrLimitNoteSize = errors.New("limit_note_size")

	// ErrLimitTotalNotes is returned when the library has reached MaxTotalNotes.
	ErrLimitTotalNotes = errors.New("limit_total_notes")

	// ErrLimitAssetSize is returned when a single asset exceeds MaxAssetSize.
	ErrLimitAssetSize = errors.New("limit_asset_size")

	// ErrLimitTotalAssets is returned when the library has reached MaxTotalAssets.
	ErrLimitTotalAssets = errors.New("limit_total_assets")

	// ErrQuotaCheckFailed is returned when the quota pre-check itself fails
	// (e.g. the data directory is unreadable).
	ErrQuotaCheckFailed = errors.New("quota_check_failed")

	// ErrLimitCycleDetected is returned by CheckStructureQuota when the
	// proposed structure contains a parent→child cycle.
	ErrLimitCycleDetected = errors.New("limit_cycle_detected")
)
