package launcher

import (
	"fmt"
	"path"

	"github.com/Fantom-foundation/lachesis-base/kvdb"
	"github.com/Fantom-foundation/lachesis-base/kvdb/cachedproducer"
	"github.com/Fantom-foundation/lachesis-base/kvdb/multidb"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"gopkg.in/urfave/cli.v1"

	"github.com/Fantom-foundation/go-opera/gossip"
	"github.com/Fantom-foundation/go-opera/integration"
)

var (
	experimentalFlag = cli.BoolFlag{
		Name:  "experimental",
		Usage: "Allow experimental DB fixing",
	}
	dbCommand = cli.Command{
		Name:        "db",
		Usage:       "A set of commands related to leveldb database",
		Category:    "DB COMMANDS",
		Description: "",
		Subcommands: []cli.Command{
			{
				Name:      "compact",
				Usage:     "Compact all databases",
				ArgsUsage: "",
				Action:    utils.MigrateFlags(compact),
				Category:  "DB COMMANDS",
				Flags: []cli.Flag{
					utils.DataDirFlag,
				},
				Description: `
opera db compact
will compact all databases under datadir's chaindata.
`,
			},
			{
				Name:      "transform",
				Usage:     "Transform DBs layout",
				ArgsUsage: "",
				Action:    utils.MigrateFlags(dbTransform),
				Category:  "DB COMMANDS",
				Flags: []cli.Flag{
					utils.DataDirFlag,
				},
				Description: `
opera db transform
will migrate tables layout according to the configuration.
`,
			},
			{
				Name:      "heal",
				Usage:     "Experimental - try to heal dirty DB",
				ArgsUsage: "",
				Action:    utils.MigrateFlags(healDirty),
				Category:  "DB COMMANDS",
				Flags: []cli.Flag{
					utils.DataDirFlag,
					experimentalFlag,
				},
				Description: `
opera db heal --experimental
Experimental - try to heal dirty DB.
`,
			},
		},
	}
)

func makeUncheckedDBsProducers(cfg *config) map[multidb.TypeName]kvdb.IterableDBProducer {
	dbsList, _ := integration.SupportedDBs(path.Join(cfg.Node.DataDir, "chaindata"), cfg.DBs.RuntimeCache)
	return dbsList
}

func makeUncheckedCachedDBsProducers(chaindataDir string) map[multidb.TypeName]kvdb.FullDBProducer {
	dbTypes, _ := integration.SupportedDBs(chaindataDir, integration.DBsCacheConfig{
		Table: map[string]integration.DBCacheConfig{
			"": {
				Cache:   1024 * opt.MiB,
				Fdlimit: uint64(utils.MakeDatabaseHandles() / 2),
			},
		},
	})
	wrappedDbTypes := make(map[multidb.TypeName]kvdb.FullDBProducer)
	for typ, producer := range dbTypes {
		wrappedDbTypes[typ] = cachedproducer.WrapAll(&integration.DummyScopedProducer{IterableDBProducer: producer})
	}
	return wrappedDbTypes
}

func makeCheckedDBsProducers(cfg *config) map[multidb.TypeName]kvdb.IterableDBProducer {
	if err := integration.CheckStateInitialized(path.Join(cfg.Node.DataDir, "chaindata"), cfg.DBs); err != nil {
		utils.Fatalf(err.Error())
	}
	return makeUncheckedDBsProducers(cfg)
}

func makeDirectDBsProducerFrom(dbsList map[multidb.TypeName]kvdb.IterableDBProducer, cfg *config) kvdb.FullDBProducer {
	multiRawDbs, err := integration.MakeDirectMultiProducer(dbsList, cfg.DBs.Routing)
	if err != nil {
		utils.Fatalf("Failed to initialize multi DB producer: %v", err)
	}
	return multiRawDbs
}

func makeDirectDBsProducer(cfg *config) kvdb.FullDBProducer {
	dbsList := makeCheckedDBsProducers(cfg)
	return makeDirectDBsProducerFrom(dbsList, cfg)
}

func makeGossipStore(producer kvdb.FlushableDBProducer, cfg *config) *gossip.Store {
	return gossip.NewStore(producer, cfg.OperaStore)
}

func compact(ctx *cli.Context) error {

	cfg := makeAllConfigs(ctx)

	rawProducer := makeDirectDBsProducer(cfg)
	for _, name := range rawProducer.Names() {
		db, err := rawProducer.OpenDB(name)
		defer db.Close()
		if err != nil {
			log.Error("Cannot open db or db does not exists", "db", name)
			return err
		}

		log.Info("Stats before compaction", "db", name)
		showLeveldbStats(db)

		log.Info("Triggering compaction", "db", name)
		for b := byte(0); b < 255; b++ {
			log.Trace("Compacting chain database", "db", name, "range", fmt.Sprintf("0x%0.2X-0x%0.2X", b, b+1))
			if err := db.Compact([]byte{b}, []byte{b + 1}); err != nil {
				log.Error("Database compaction failed", "err", err)
				return err
			}
		}

		log.Info("Stats after compaction", "db", name)
		showLeveldbStats(db)
	}

	return nil
}

func showLeveldbStats(db ethdb.Stater) {
	if stats, err := db.Stat("leveldb.stats"); err != nil {
		log.Warn("Failed to read database stats", "error", err)
	} else {
		fmt.Println(stats)
	}
	if ioStats, err := db.Stat("leveldb.iostats"); err != nil {
		log.Warn("Failed to read database iostats", "error", err)
	} else {
		fmt.Println(ioStats)
	}
}
