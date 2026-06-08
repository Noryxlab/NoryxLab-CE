# CE and EE boundaries

NoryxLab CE contains the standard data platform capabilities. Enterprise-only
behavior is activated through `backend/internal/edition` hooks and must remain
disabled by the default CE hooks.

## Dataset boundary

CE supports:

- platform-managed MinIO datasets
- standard external S3 datasets
- owner, writer, and reader ACLs
- project attachment, workspace mounting, preview, download, and editing

HDS dataset management is an Enterprise Edition capability:

- CE rejects creation requests with `classification=hds`
- CE hides historical HDS records from user and administration inventories
- CE blocks access, assignment, ACL management, and S3 operations for HDS records
- EE enables `edition.FeatureHDSDatasets` and supplies the regulated policies,
  deployment controls, audit requirements, and user interface
