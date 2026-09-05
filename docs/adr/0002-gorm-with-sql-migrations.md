# Use GORM adapters with manually applied versioned SQL

Persistence adapters use GORM for query implementation, while versioned SQL files remain the only schema source. Operators review and execute those files manually, one version at a time, as a separate deployment step; the repository does not provide an automatic migration runner. GORM models stay separate from domain entities and HTTP DTOs, and services never call AutoMigrate.

Manual execution trades automatic database version discovery for explicit operational control. Every release must therefore record the executed filename, checksum, environment, operator, result, and verification evidence. Applied SQL files are immutable, and corrections use a new forward-fix version.
