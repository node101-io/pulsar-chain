package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/bridge module sentinel errors
var (
	ErrInvalidSigner                          = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	ErrUnspecified                            = errors.Register(ModuleName, 1101, "unspecified action type")
	ErrNotEnoughBalance                       = errors.Register(ModuleName, 1102, "not enough balance")
	ErrBankKeeperNotConfigured                = errors.Register(ModuleName, 1103, "bank keeper is not configured")
	ErrKeyRegistryKeeperNotConfigured         = errors.Register(ModuleName, 1104, "keyregistry keeper is not configured")
	ErrMinaBlockNotFinalized                  = errors.Register(ModuleName, 1105, "mina block not finalized")
	ErrArchiveWrapperQueryClientNotConfigured = errors.Register(ModuleName, 1106, "archive wrapper query client is not configured")

	ErrInvalidMinaBlockHeight     = errors.Register(ModuleName, 1107, "mina block height must be greater than 0")
	ErrMinaBlockHeightMustAdvance = errors.Register(ModuleName, 1108, "mina block height must advance past latest fetched height")
	ErrInvalidMinaBlockRange      = errors.Register(ModuleName, 1109, "invalid mina block range")

	ErrConfirmationDepthMustBeGreaterThanZero = errors.Register(ModuleName, 1110, "confirmation_depth must be greater than 0")
	ErrEmptyContractAddress                   = errors.Register(ModuleName, 1111, "contract_address must not be empty")
	ErrInvalidContractAddress                 = errors.Register(ModuleName, 1112, "invalid contract_address")

	ErrInvalidBridgeStateHeight           = errors.Register(ModuleName, 1113, "bridge state height must be non-negative")
	ErrActionsReducedRootSnapshotNotFound = errors.Register(ModuleName, 1114, "actions reduced root snapshot not found")

	ErrArchiveWrapperQueryTimeout   = errors.Register(ModuleName, 1115, "wrapper query time-out")
	ErrArchiveWrapperQueryCancelled = errors.Register(ModuleName, 1116, "wrapper query cancelled")

	ErrInvalidLatestFetchedMinaHeight              = errors.Register(ModuleName, 1117, "invalid latest_fetched_mina_height")
	ErrEmptyActionsReducedRootSnapshots            = errors.Register(ModuleName, 1118, "actions_reduced_root_snapshots must not be empty")
	ErrInvalidActionsReducedRootSnapshotHeight     = errors.Register(ModuleName, 1119, "invalid actions_reduced_root_snapshot cosmos_block_height")
	ErrInvalidActionsReducedRoot                   = errors.Register(ModuleName, 1120, "invalid actions_reduced_root")
	ErrActionsReducedRootSnapshotsMustStartAtZero  = errors.Register(ModuleName, 1121, "actions_reduced_root_snapshots must start at cosmos block height 0")
	ErrInvalidInitialActionsReducedRoot            = errors.Register(ModuleName, 1122, "invalid initial actions_reduced_root")
	ErrActionsReducedRootSnapshotsMustBeIncreasing = errors.Register(ModuleName, 1123, "actions_reduced_root_snapshots must be strictly increasing by cosmos_block_height")

	ErrStartBlockHeightMustBeGreaterThanZero   = errors.Register(ModuleName, 1124, "start_block_height must be greater than 0")
	ErrLatestFetchedMinaHeightBeforeStartBlock = errors.Register(ModuleName, 1125, "latest_fetched_mina_height must be at or after start_block_height - 1")

	ErrMaxBlockRangeMustBeGreaterThanZero = errors.Register(ModuleName, 1126, "max_block_range must be greater than 0")
	ErrMinaBlockRangeTooLarge             = errors.Register(ModuleName, 1127, "mina block range exceeds max_block_range")

	ErrInvalidArchiveWrapperGRPCAddress = errors.Register(ModuleName, 1128, "wrapper_grpc_address must be a loopback host:port address")

	ErrNilAction                = errors.Register(ModuleName, 1129, "action is nil")
	ErrInvalidActionBlockHeight = errors.Register(ModuleName, 1130, "invalid action block_height")
	ErrInvalidActionAmount      = errors.Register(ModuleName, 1131, "invalid action amount")
	ErrInvalidActionType        = errors.Register(ModuleName, 1132, "invalid action type")
	ErrInvalidActionFeePayer    = errors.Register(ModuleName, 1133, "invalid action fee_payer")
	ErrActionToFieldFailed      = errors.Register(ModuleName, 1134, "failed to convert action to field")
	ErrArchiveWrapperNotReady   = errors.Register(ModuleName, 1135, "archive wrapper is not ready")

	ErrActionOutsideRequestedRange = errors.Register(ModuleName, 1136, "action outside the requested range")

	ErrTooManyActionsReducedRootSnapshots                        = errors.Register(ModuleName, 1137, "too many actions_reduced_root_snapshots")
	ErrCurrentActionsReducedRootMismatch                         = errors.Register(ModuleName, 1138, "bridge_state.current_actions_reduced_root must match latest actions_reduced_root snapshot")
	ErrActionsReducedRootSnapshotWindowSizeMustBeGreaterThanZero = errors.Register(ModuleName, 1139, "actions_reduced_root_snapshot_window_size must be greater than 0")
)
