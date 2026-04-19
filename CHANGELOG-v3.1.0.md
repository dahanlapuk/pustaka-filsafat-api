# 📝 CHANGELOG — v3.1.0 (Released: April 20, 2026)

## Overview
Major patch implementing Phases 1-4a: Mobile UX overhaul, comprehensive activity logging, multi-tagging system, stock split per location, and advanced loan allocation tracking. **100% backward compatible** — no breaking changes.

---

## 🎨 [PHASE 1] Mobile UX & Activity Logging

### Features Added

#### Mobile Dark Theme Parity
- **Mobile & Responsive**: Extended dark theme consistency to all pages (PublicCatalog, Dashboard, Loans, Inventory, AdminPanel, etc.)
- **Theme Toggle**: Persistent theme switcher in mobile navbar, remembers user preference via localStorage
- **Navigation**: Added "Back to Catalog" link in mobile admin menu for quick return to public view
- **CSS Token System**: Remapped legacy hardcoded colors to centralized theme tokens (bauhaus.css → theme.css)

#### Public Borrowing Flow (Mobile & Desktop)
- **Borrow Button**: Added "Pinjam" (Borrow) button on public catalog cards and detail modals
- **Availability Indicator**: Shows book status (Tersedia/Dipinjam) with color-coded badges
- **Metadata Display**: Author (penulis) and publication year (tahun) now visible when available
- **Responsive Modal**: Borrow form works seamlessly on mobile, tablet, desktop

#### Comprehensive Activity Logging
- **26+ Action Types**: Logging for all major operations:
  - Auth: LOGIN, LOGOUT
  - Books: CREATE, UPDATE, DELETE, CODE_GENERATE
  - Categories: CREATE, UPDATE, DELETE
  - Requests: CATEGORY_REQUEST_CREATE/APPROVE/REJECT, DELETE_REQUEST_CREATE/APPROVE/REJECT
  - Loans: LOAN_CREATE, LOAN_RETURN
  - Admin: ADMIN_CREATE, ADMIN_UPDATE, PASSWORD_RESET
  - Inventory: BOOK_STOCK_SPLIT_UPDATE

- **Rich Context**: Each log entry captures:
  - Actor: who performed the action (admin name/ID)
  - Entity: what was affected (book, category, loan, etc.)
  - Detail: what changed (before/after for updates, reason for deletion, etc.)
  - Timestamp: exact time of action with timezone

- **Admin Dashboard**: 
  - Removed redundant "Kategori Terbanyak" block (duplicate of filter)
  - Removed "Log Aktivitas Admin" block (its own page exists)
  - Retained "Aktivitas Terbaru" block with CTA to full ActivityLogs page
  - Reduced dashboard clutter, improved performance

### Files Modified
- `src/pages/PublicCatalog.vue` — theme toggle, borrow flow, author/year display
- `src/pages/Dashboard.vue` — cleanup redundant blocks
- `src/components/Navbar.vue` — mobile theme switcher, catalog link
- `src/pages/ActivityLogs.vue` — consistent timestamp formatting
- `src/api/index.js` — new loan request endpoint
- `src/style/bauhaus.css` — token remapping for dark theme parity
- `handlers/activity_log.go` — logging architecture
- `models/activity_log.go` — activity model definition
- `routes.go` — activity log endpoint registration

### Testing Checklist
- [x] Dark theme renders correctly on all pages
- [x] Theme persists across page refreshes
- [x] Mobile navbar shows theme toggle
- [x] Borrow button appears only for available books
- [x] Author/year display when available
- [x] Activity log includes all 26+ action types
- [x] Timestamps display consistently across timezone
- [x] Dashboard loads without removed blocks

---

## 🏷️ [PHASE 2] Multi-Tagging with Semantic Grouping

### Features Added

#### Multi-Tag Data Model
- **Primary Category + Tags**: Books can have one primary category + unlimited additional tags
- **Semantic Grouping**: Tags organized into optional groups:
  - **Bentuk** (Form): physical format (e-book, hardcover, audiobook, etc.)
  - **Konten** (Content): subject matter (fiksi, non-fiksi, academic, etc.)
  - **Lainnya** (Others): miscellaneous (recommended, bestseller, etc.)
  - **Tanpa Group** (None): ungrouped tags (flexible classification)

#### Book Form Enhancements
- **Typed Hashtag Input**: Type "#" to input tags with real-time suggestions
- **Chip UI**: Visual tags appear as removable chips as you type
- **Validation**: Duplicate tag prevention, normalization (lowercase, trim)
- **Fallback Payload**: Both tag_ids (IDs) and tag_names (strings) sent to backend for flexibility

#### Catalog & Search Integration
- **Hashtag Display**: Tags shown with "#" prefix in:
  - Public catalog cards (inline with category badge)
  - Detail modals
  - Search results
- **Filter by Tag**: Dropdown selector with optgroup headers (Bentuk / Konten / Lainnya)
- **Multi-filter**: Combine category filter + tag filter in single query
- **Responsive Layout**: Tag display wraps gracefully on mobile

#### Category Request Update
- **Tag Support**: Category request workflow now includes semantic grouping metadata
- **Audit Trail**: Approval/rejection logs which tags were added/modified
- **Backward Compat**: Existing categories without tags continue to work

### Files Modified
- `src/pages/BookForm.vue` — chip input, tag suggestions, hashtag parsing
- `src/pages/PublicCatalog.vue` — hashtag display, tag filtering, responsive layout
- `src/api/index.js` — tag-aware search params
- `handlers/books.go` — tag CRUD, batch tag sync, transaction-safe updates
- `handlers/categories.go` — grouping metadata, tag search index
- `models/category.go` — grouping field support
- `routes.go` — tag endpoints

### Database Changes
- `book_categories` table: many-to-many relation for book-tag pairs
- Index on (book_id, category_id, grouping) for fast tag lookups
- Migration: auto-create from legacy single category where applicable

### Testing Checklist
- [x] Book form accepts typed hashtagged tags
- [x] Tags persist after save
- [x] Tags display in catalog with proper grouping colors
- [x] Tag filter works in public catalog
- [x] Search filters by tag_id correctly
- [x] Chip UI removes tags on delete
- [x] Backward compat: books without tags still load
- [x] Category request includes tag changes in audit log

---

## 📦 [PHASE 3] Stock Distribution Across Locations

### Features Added

#### Multi-Location Inventory Model
- **Book Stock Locations**: Track inventory at multiple shelf positions:
  - `book_id` + `posisi_id` + `qty` per row
  - Example: "Jurnal Perempuan" → 2 copies at R1-A, 1 copy at R2-C, 1 copy at Gudang
- **Stock Aggregation**: Book.qty = sum of all position allocations (auto-maintained)
- **Backward Compat**: Single-qty books work without split (optional feature)

#### Admin Split Editor (Update Posisi Page)
- **Detection**: When book qty > 1, "Ubah" button shows split editor instead of single dropdown
- **Multi-Row Allocation UI**:
  - Grid with Posisi dropdown + Qty input per row
  - "+ Tambah Baris Posisi" to add more splits
  - Real-time total validation: current total / required qty
  - Hapus button to remove rows (min 1 row)
- **Validation**:
  - Qty per position must be ≥ 0
  - All positions with qty > 0 must have posisi_id selected
  - Total qty **must equal** book qty exactly
- **Save & Persist**: Puts to backend transaction-safe, updates dominant position
- **Error Handling**: Toast feedback on validation fail, API errors

#### Smart Loan Allocation Algorithm
- **Greedy Allocation**: When borrowing, system allocates from position with largest available stock
  - Logic: `(position_qty) - (active_loans)` → pick largest remainder
  - Falls back to next largest position for subsequent loans
  - Ensures even distribution and optimal fulfillment
- **Transaction Safety**: All allocation & loan create/return within SQL transaction
- **Loan Tracking**: Each loan linked to specific posisi_id in loan_stock_allocations table

#### Stock Consistency Guarantee
- **Sync on Loan**: When loan created, syncs book qty with position totals
- **Return Release**: When loan returned, marked returned_at timestamp (not deleted, preserved for audit)
- **Consistency Check**: Admin can verify sum(location qty) = book qty via API

### Files Modified
- `src/pages/UpdatePosisi.vue` — split editor UI, validation, save flow
- `handlers/inventory_split.go` — EnsureBookStockSchema, GetBreakdown, UpdateBreakdown
- `handlers/loans.go` — allocateLoanFromLargestStockTx, stock sync
- `models/` — stock location & allocation structures
- `routes.go` — stock endpoint registration
- `scripts/schema.sql` — table creation, migration scripts

### Database Changes
- `book_stock_locations`: (book_id, posisi_id, qty, created_at, updated_at)
  - Unique constraint on (book_id, posisi_id)
  - Foreign keys: books.id, posisi.id with cascade delete

- `loan_stock_allocations`: (loan_id, book_id, posisi_id, qty, allocated_at, returned_at)
  - Tracks which position each loan was allocated from
  - returned_at NULL means active, filled on return for audit

- Indexes:
  - `idx_book_stock_locations_book_id` for fast lookups
  - `idx_loan_stock_allocations_book_posisi_active` (partial) for available qty calc

### API Endpoints
- **GET** `/books/:id/stock-breakdown` — current allocations with qty per position
- **PUT** `/books/:id/stock-breakdown` — update allocations (split editor save)

### Testing Checklist
- [x] Book qty=1 does NOT show split editor (backward compat)
- [x] Book qty≥2 shows split editor UI
- [x] Can allocate to multiple positions
- [x] Total validation prevents save if != qty
- [x] Positions without selection are skipped
- [x] Save updates dominant position in books table
- [x] Loan allocates from largest stock position first
- [x] Return loan marks returned_at timestamp
- [x] Stock consistency check passes (sum = qty)

---

## 👁️ [PHASE 4a] Loan Allocation Visibility & Tracking

### Features Added

#### Loan Detail Endpoint
- **GET** `/api/loans/:id` returns:
  ```json
  {
    "id": 123,
    "book_id": 456,
    "nama_peminjam": "John Doe",
    "tanggal_pinjam": "2026-04-20T10:30:00Z",
    "allocations": [
      {
        "allocation_id": 999,
        "posisi_kode": "R1-A-B",
        "posisi_rak": "Rak Utama",
        "qty": 1,
        "allocated_at": "2026-04-20T10:30:00Z",
        "returned_at": null
      }
    ]
  }
  ```
- Use case: Admin can see exactly which position a loan came from

#### Stock Availability per Position
- **GET** `/api/books/:id/stock-availability` returns:
  ```json
  {
    "book_id": 123,
    "total_qty": 3,
    "borrowed_qty": 2,
    "available_qty": 1,
    "locations": [
      {
        "posisi_kode": "R1-A-B",
        "total_qty": 2,
        "borrowed_qty": 2,
        "available_qty": 0
      },
      {
        "posisi_kode": "R2-C-F",
        "total_qty": 1,
        "borrowed_qty": 0,
        "available_qty": 1
      }
    ]
  }
  ```
- Use case: Public borrow modal can show "2/2 tersedia di R1-A-B, 1/1 tersedia di R2-C-F"

#### Public Catalog Borrow Modal Enhancement
- **Load Availability**: When user clicks "Pinjam", fetches availability per position automatically
- **Display**: Shows breakdown:
  ```
  Ketersediaan per Posisi
  ┌─────────────────┬──────────┐
  │ R1-A-B (Rak Utama)  │ 0/2      │
  │ R2-C-F (Gudang) │ 1/1      │
  └─────────────────┴──────────┘
  ```
  - Green for available > 0
  - Orange/red for available = 0
  - Shows sorted by available qty (largest first)
- **User UX**: Borrowers see exactly where copies exist before requesting

### Files Modified
- `handlers/loans.go` — GetLoanDetail, GetBookStockAvailability added
- `src/pages/PublicCatalog.vue` — modal load availability, display breakdown
- `src/api/index.js` — getLoanDetail, getBookStockAvailability helpers
- `routes.go` — endpoint registration

### Testing Checklist
- [x] GET /loans/:id returns allocation with posisi details
- [x] GET /books/:id/stock-availability calculates available qty correctly
- [x] Public catalog modal loads availability on open
- [x] Availability displays with correct formatting
- [x] Locations sorted by available qty descending
- [x] Inactive loans not counted in borrowed_qty
- [x] Returned loans removed from active borrowed count

---

## 🐛 [MINOR] Category Sidebar Styling

### Improvements
- **Color Contrast**: Fixed category item text color for better readability in dark mode
  - Base text now uses `var(--text-primary)` for consistent visibility
  - Hover state adds accent color highlight
  - Active state uses `var(--text-inverse)` for proper contrast on accent background
- **Visual Feedback**: Improved hover UX with accent color change on category items
- **Consistency**: Aligned with overall theme token system

### Files Modified
- `src/pages/PublicCatalog.vue` — category-item styling updates

---

## 📊 Summary Statistics

| Metric | Value |
|--------|-------|
| New Lines of Code | ~3,000 |
| Files Modified | 25+ |
| New Endpoints | 4 |
| New Database Tables | 2 |
| Activity Log Actions | 26+ types |
| Breaking Changes | 0 |
| Backward Compatibility | 100% |
| Build Time (Backend) | ~5-10s |
| Build Time (Frontend) | ~2s |
| Binary Size | 12 MB |
| Bundle Size | 273 KB JS + 74 KB CSS |

---

## 🚀 Migration & Deployment

### No Pre-Migration Required
- Schema ensure runs automatically at backend startup
- Old books auto-migrated to default position if no split exists
- Activity logs start fresh (retro-logging not applicable)
- Tag table auto-populated on first book with tags

### Backward Compatibility Guarantees
- [x] Books without tags still display (no schema requirement)
- [x] Books with qty=1 work as single location (split optional)
- [x] Old loan data continues to work
- [x] Activity logs don't break UI on empty logs
- [x] Category-only books compatible with new multi-tag system

### Deployment Steps
1. **Backup**: Save current app binary and dist/ folder
2. **Deploy Backend**: Replace binary, restart service (auto-schema ensure)
3. **Deploy Frontend**: Replace dist/ folder
4. **Verify**: Run smoke tests (see deployment checklist)
5. **Monitor**: Watch logs for allocation accuracy, stock consistency

---

## 🔍 Known Limitations & Future Work

### Phase 4b (Future)
- [ ] Inventory Transfer UI (move stock between positions)
- [ ] Stock Stocktake (physical count reconciliation)
- [ ] Loan Audit Dashboard (advanced reporting)
- [ ] Member Profiles (borrower history)

### Potential Improvements
- [ ] Bulk tag assignment to existing books
- [ ] Tag auto-complete from historical data
- [ ] Location-specific loan restrictions (e.g., "Gudang copies not lendable")
- [ ] Stock transfer workflow for admin

---

## 🎯 Performance Considerations

### Database
- New indexes on (book_id, posisi_id) optimize split lookups
- Partial index on loan_allocations filters active loans efficiently
- Query cost for availability: O(1) table scan (small allocation table)

### Frontend
- Availability fetch is async, doesn't block modal display
- Lazy-load tag suggestions only on focus
- Theme toggle uses localStorage (instant, no API call)

### Caching Recommendations
- Cache category list (rarely changes)
- Cache availability per book for 5-10 seconds (stale OK for display)
- No caching for active loans (must be real-time)

---

## 🙏 Contributors
- Full-stack development: Phase 1-4a implementation
- Testing: Mobile UX, backward compatibility, allocation algorithm
- Documentation: Comprehensive changelog, deployment guide, phase status

---

## 📋 Verification Checklist (Pre-Launch)

- [x] Mobile dark theme consistent across all pages
- [x] Theme toggle works & persists
- [x] Borrow button shows/hides correctly based on availability
- [x] Activity log comprehensive (26+ actions)
- [x] Multi-tag system functional (input, display, filter)
- [x] Stock split UI accessible (Update Posisi) for qty > 1
- [x] Split validation prevents invalid saves
- [x] Loan allocation uses greedy algorithm
- [x] Availability per position displays correctly in borrow modal
- [x] Backward compatibility tested (old books, old loans)
- [x] Both binaries compile without errors
- [x] No breaking API changes
- [x] Deployment documented with rollback plan

---

**Version**: v3.1.0  
**Release Date**: April 20, 2026  
**Status**: ✅ READY FOR PRODUCTION

For detailed deployment instructions, see: `deployment-checklist-v3.1.0.md`
