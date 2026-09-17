-- Model plant lama digantikan mapping_nodes dan mapping_edges di migrasi 53,
-- yang sudah memindahkan isinya dengan id yang sama, jadi odp_id milik ONT
-- tetap sahih dan kolomnya tidak ikut dihapus.
--
-- Kunci asing ini harus dilepas lebih dulu: onts tidak ikut dihapus, jadi
-- fk_onts_odp (migrasi 39) masih menggantung ke odps dan membuat DROP TABLE
-- gagal -- yang berarti API menolak menyala setelah deploy.
ALTER TABLE onts DROP CONSTRAINT IF EXISTS fk_onts_odp;

DROP TABLE IF EXISTS odc_feeds;
DROP TABLE IF EXISTS odps;
DROP TABLE IF EXISTS odcs;
