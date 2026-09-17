-- mapping_nodes dan mapping_edges: peta jaringan yang bisa diisi dari lapangan.
--
-- AutoMigrate membuat tabelnya dari model; berkas ini menambahkan yang tag model
-- tidak bisa nyatakan, lalu memindahkan isi model plant lama.
--
-- Tidak ada foreign key dari edge ke node. Sebuah kabel digambar lebih dulu dan
-- ujungnya diberi nama menyusul, dan menghapus satu node tidak boleh membuat
-- seluruh jalur yang sudah ditarik ikut hilang tanpa jejak.

ALTER TABLE mapping_nodes DROP CONSTRAINT IF EXISTS mapping_nodes_type_valid;
ALTER TABLE mapping_nodes ADD CONSTRAINT mapping_nodes_type_valid
    CHECK (type IN ('server', 'odc', 'odp', 'ont'));

ALTER TABLE mapping_edges DROP CONSTRAINT IF EXISTS mapping_edges_fiber_type_valid;
ALTER TABLE mapping_edges ADD CONSTRAINT mapping_edges_fiber_type_valid
    CHECK (fiber_type IS NULL OR fiber_type IN (
        'feeder', 'distribution', 'drop',
        'odp_to_odp', 'odp_to_odp_ratio', 'odc_to_odc', 'odc_to_odc_ratio'));

-- Dibaca saat menggambar kabel dan saat menghitung slot terpakai.
CREATE INDEX IF NOT EXISTS idx_mapping_edges_source ON mapping_edges (source);
CREATE INDEX IF NOT EXISTS idx_mapping_edges_target ON mapping_edges (target);
CREATE INDEX IF NOT EXISTS idx_mapping_nodes_type ON mapping_nodes (type);

-- Pindahkan ODC dan ODP lama. Keduanya hanya berisi satu baris di produksi,
-- tapi memindahkannya berarti tidak ada yang hilang di instalasi mana pun.
--
-- code tidak unik — tidak ada constraint-nya di tag model, di migration 39/40,
-- ataupun di CreateODC/CreateODP — jadi dua baris berkode sama menabrak
-- uniqueIndex pada node_id dan menggagalkan migrasi ini, yang lewat
-- RunSQLMigrations menggagalkan startup API sendiri (log.Fatal di
-- cmd/api/main.go). row_number() memberi baris pertama id yang bersih dan
-- hanya baris yang benar-benar tabrakan yang mendapat akhiran dari id-nya
-- sendiri. Postgres menolak window function langsung di WHERE, jadi id yang
-- sudah didisambiguasi dihitung sekali di CTE lalu dipakai ulang di SELECT
-- maupun di WHERE NOT EXISTS — guard replay yang sama seperti sebelumnya,
-- kini dicocokkan terhadap id akhir yang benar-benar disimpan.
--
-- Baris tanpa koordinat memakai (0,0) karena kolomnya NOT NULL, tapi (0,0)
-- ada di lepas pantai Afrika sementara seluruh plant ISP ini ada di 95-141
-- derajat bujur timur — baris begini pasti salah, dan notes diberi tanda
-- supaya kelihatan di daftar alih-alih diam-diam terlihat seperti posisi asli.
--
-- id dipertahankan dari odcs/odps sendiri, bukan gen_random_uuid(): onts.odp_id
-- masih menyimpan odps.id lama, migrasi ini tidak menyentuh tabel onts sama
-- sekali, dan odps akan di-drop di task berikutnya. id baru berarti setiap ONT
-- yang sudah punya odp_id kehilangan pasangannya secara diam-diam begitu odps
-- hilang, tanpa tabel tersisa untuk memulihkannya. odcs.id dan odps.id adalah
-- dua ruang UUID terpisah yang bertemu di satu primary key mapping_nodes.id;
-- tabrakan di antara keduanya nyaris mustahil dan akan menggagalkan INSERT
-- dengan keras, bukan merusak data diam-diam.
-- odcs.code is itself varchar(64), so 'ODC-' || code alone can already exceed
-- mapping_nodes.node_id's varchar(64) before any collision suffix is added.
-- left(..., 64 - 9) reserves room for that suffix ('-' plus 8 hex characters)
-- so the final value never overflows. Truncating the prefix+code instead of
-- the finished string matters: truncating after appending the suffix would,
-- for a long enough code, cut the suffix off rather than the code, defeating
-- the very thing it disambiguates.
WITH odc_ids AS (
    SELECT o.*,
           left('ODC-' || o.code, 64 - 9) || CASE
               WHEN row_number() OVER (PARTITION BY o.code ORDER BY o.created_at, o.id) > 1
               THEN '-' || left(o.id::text, 8) ELSE '' END AS mapped_node_id
    FROM odcs o
)
INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, notes, created_at, updated_at)
SELECT id, mapped_node_id, 'odc', code,
       COALESCE(latitude, 0), COALESCE(longitude, 0), 0,
       CASE WHEN latitude IS NULL OR longitude IS NULL
            THEN '[koordinat belum diisi] ' ELSE '' END || COALESCE(notes, ''),
       created_at, updated_at
FROM odc_ids
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = odc_ids.mapped_node_id);

-- Same overflow guard as odc_ids above, applied to the ODP prefix.
WITH odp_ids AS (
    SELECT p.*,
           left('ODP-' || p.code, 64 - 9) || CASE
               WHEN row_number() OVER (PARTITION BY p.code ORDER BY p.created_at, p.id) > 1
               THEN '-' || left(p.id::text, 8) ELSE '' END AS mapped_node_id
    FROM odps p
)
INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, notes, created_at, updated_at)
SELECT id, mapped_node_id, 'odp', code,
       COALESCE(latitude, 0), COALESCE(longitude, 0), COALESCE(port_count, 0),
       CASE WHEN latitude IS NULL OR longitude IS NULL
            THEN '[koordinat belum diisi] ' ELSE '' END || COALESCE(notes, ''),
       created_at, updated_at
FROM odp_ids
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = odp_ids.mapped_node_id);
