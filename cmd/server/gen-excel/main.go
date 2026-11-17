package main

import (
	"flag"
	"log"

	"smartcoop/internal/etl/excelgen"
)

func main() {
	out := flag.String("out", "data_koperasi_lama.xlsx", "path file output")
	items := flag.Int("items", 300, "jumlah barang")
	months := flag.Int("months", 6, "jumlah bulan riwayat penjualan per barang")
	members := flag.Int("members", 30, "jumlah anggota")
	dirty := flag.Bool("dirty", true, "sisipkan data kotor (baris kosong, harga berformat teks, duplikat)")
	seed := flag.Int64("seed", 42, "seed random agar hasil bisa diulang")
	flag.Parse()

	err := excelgen.WriteFile(*out, excelgen.Options{
		Items:   *items,
		Months:  *months,
		Members: *members,
		Dirty:   *dirty,
		Seed:    *seed,
	})
	if err != nil {
		log.Fatalf("gagal membuat excel: %v", err)
	}
	log.Printf("file dibuat: %s", *out)
}