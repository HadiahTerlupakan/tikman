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
INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, notes, created_at, updated_at)
SELECT gen_random_uuid(), 'ODC-' || o.code, 'odc', o.code,
       COALESCE(o.latitude, 0), COALESCE(o.longitude, 0), 0,
       COALESCE(o.notes, ''), o.created_at, o.updated_at
FROM odcs o
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = 'ODC-' || o.code);

INSERT INTO mapping_nodes (id, node_id, type, name, latitude, longitude, capacity, notes, created_at, updated_at)
SELECT gen_random_uuid(), 'ODP-' || p.code, 'odp', p.code,
       COALESCE(p.latitude, 0), COALESCE(p.longitude, 0), COALESCE(p.port_count, 0),
       COALESCE(p.notes, ''), p.created_at, p.updated_at
FROM odps p
WHERE NOT EXISTS (SELECT 1 FROM mapping_nodes m WHERE m.node_id = 'ODP-' || p.code);
