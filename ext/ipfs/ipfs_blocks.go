// +build IPFS_FILES

package ipfs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	ufsio "github.com/ipfs/go-unixfs/io"

	chunker "github.com/ipfs/go-ipfs-chunker"

	"github.com/ipfs/go-bitswap"
	"github.com/ipfs/go-bitswap/network"
	"github.com/ipfs/go-blockservice"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	provider "github.com/ipfs/go-ipfs-provider"
	"github.com/ipfs/go-ipfs-provider/queue"
	"github.com/ipfs/go-ipfs-provider/simple"
	"github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs/importer/balanced"
	"github.com/ipfs/go-unixfs/importer/helpers"
	"github.com/ipfs/go-unixfs/importer/trickle"
	"github.com/multiformats/go-multihash"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	ipld "github.com/ipfs/go-ipld-format"
)

type IPFS_BS struct {
	*IPFS

	// Also provide block - since we pay the DHT price.
	// File serving
	ipld.DAGService
	bstore          blockstore.Blockstore
	bserv           blockservice.BlockService
	reprovider      provider.System

}

// Specific to file serving
func init() {
	if os.Getenv("IPFS_DIR") != "" {
		ipld.Register(cid.DagProtobuf, merkledag.DecodeProtobufBlock)

		ipld.Register(cid.Raw, merkledag.DecodeRawBlock)
		ipld.Register(cid.DagCBOR, cbor.DecodeBlock) // need to decode CBOR

		blockInit = func(node *IPFS) {
			p2p := &IPFS_BS{IPFS: node}
			p2p.setupBlockstore()
			p2p.setupBlockService()
			p2p.setupDAGService()
			p2p.setupReprovider()
			// bafybeickrjm6kmamm5obkqszwvqfnstk4diu6e6ztswxoti5ee3jwyt6ym
			buf := bytes.NewReader([]byte("IPFS test"))
			n, _ := p2p.AddFile(context.Background(), buf, nil)
			if n != nil {
				log.Println("Added ", n.Cid())
			}
		}
	}
}

func (p *IPFS_BS) setupBlockstore() error {
	bs := blockstore.NewBlockstore(p.store)
	bs = blockstore.NewIdStore(bs)
	cachedbs, err := blockstore.CachedBlockstore(p.ctx, bs, blockstore.DefaultCacheOpts())
	if err != nil {
		return err
	}
	p.bstore = cachedbs
	return nil
}

func (p *IPFS_BS) setupBlockService() error {
	// Second param is the ContentRouting -
	// for the real IPFS net it must be the DHT.
	// Used for 'Provide(CID)' and 'FindProvidersAsync'
	// local discovery can also provide this.
	bswapnet := network.NewFromIpfsHost(p.Host, p.DHT)
	bswap := bitswap.New(p.ctx, bswapnet, p.bstore)
	p.bserv = blockservice.New(p.bstore, bswap)
	return nil
}

func (p *IPFS_BS) setupDAGService() error {
	p.DAGService = merkledag.NewDAGService(p.bserv)
	return nil
}

func (p *IPFS_BS) setupReprovider() error {
	// Can also use fsrepo

	queue, err := queue.NewQueue(p.ctx, "repro", p.store)
	if err != nil {
		return err
	}

	prov := simple.NewProvider(
		p.ctx,
		queue,
		p.DHT,
	)

	reprov := simple.NewReprovider(
		p.ctx,
		12*time.Hour,
		p.DHT,
		simple.NewBlockstoreProvider(p.bstore),
	)

	p.reprovider = provider.NewSystem(prov, reprov)
	p.reprovider.Run()
	return nil
}

func (p *IPFS_BS) autoclose() {
	<-p.ctx.Done()
	p.reprovider.Close()
	p.bserv.Close()
}

// Session returns a session-based NodeGetter.
func (p *IPFS_BS) Session(ctx context.Context) ipld.NodeGetter {
	ng := merkledag.NewSession(ctx, p.DAGService)
	if ng == p.DAGService {
		logger.Warn("DAGService does not support sessions")
	}
	return ng
}

// AddParams contains all of the configurable parameters needed to specify the
// importing process of a file.
type AddParams struct {
	Layout    string
	Chunker   string
	RawLeaves bool
	Hidden    bool
	Shard     bool
	NoCopy    bool
	HashFun   string
}

// AddFile chunks and adds content to the DAGService from a reader. The content
// is stored as a UnixFS DAG (default for IPFS). It returns the root
// ipld.Node.
func (p *IPFS_BS) AddFile(ctx context.Context, r io.Reader, params *AddParams) (ipld.Node, error) {
	if params == nil {
		params = &AddParams{}
	}
	if params.HashFun == "" {
		params.HashFun = "sha2-256"
	}

	prefix, err := merkledag.PrefixForCidVersion(1)
	if err != nil {
		return nil, fmt.Errorf("bad CID Version: %s", err)
	}

	hashFunCode, ok := multihash.Names[strings.ToLower(params.HashFun)]
	if !ok {
		return nil, fmt.Errorf("unrecognized hash function: %s", params.HashFun)
	}
	prefix.MhType = hashFunCode
	prefix.MhLength = -1

	dbp := helpers.DagBuilderParams{
		Dagserv:    p,
		RawLeaves:  params.RawLeaves,
		Maxlinks:   helpers.DefaultLinksPerBlock,
		NoCopy:     params.NoCopy,
		CidBuilder: &prefix,
	}

	chnk, err := chunker.FromString(r, params.Chunker)
	if err != nil {
		return nil, err
	}
	dbh, err := dbp.New(chnk)
	if err != nil {
		return nil, err
	}

	var n ipld.Node
	switch params.Layout {
	case "trickle":
		n, err = trickle.Layout(dbh)
	case "balanced", "":
		n, err = balanced.Layout(dbh)
	default:
		return nil, errors.New("invalid Layout")
	}
	return n, err
}

// GetFile returns a reader to a file as identified by its root CID. The file
// must have been added as a UnixFS DAG (default for IPFS).
func (p *IPFS_BS) GetFile(ctx context.Context, c cid.Cid) (ufsio.ReadSeekCloser, error) {
	n, err := p.Get(ctx, c)
	if err != nil {
		return nil, err
	}
	return ufsio.NewDagReader(ctx, n, p)
}
