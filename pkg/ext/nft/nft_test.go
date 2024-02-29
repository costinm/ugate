package nft

import (
	"log"
	"net"
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/binaryutil"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"
)

func TestNFT(t *testing.T) {
	nt, err := nftables.New()
	if err != nil {
		t.Fatal()
	}
	//nt.AddRule(&nftables.Rule{})

	//cl, err := nt.ListChains()
	//if err != nil {
	//	t.Fatal(err)
	//}
	//for _, c := range cl {
	//	log.Println("CHAIN", c.Table.Name, c.Policy, c.Type, c.Hooknum, c.Priority)
	//	rl, err := nt.GetRules(c.Table, c)
	//	if err != nil {
	//		t.Fatal(err)
	//	}
	//	for _, cr := range rl {
	//		log.Println(cr.Exprs, cr.UserData)
	//	}
	//}

	ft := &nftables.Table{Name: "costin", Family: nftables.TableFamilyIPv4}
	nt.AddTable(ft)
	if err := nt.Flush(); err != nil {
		t.Fatal(err)
	}

	nt.AddChain(&nftables.Chain{Name: "divert", Table: ft, Type: nftables.ChainTypeFilter})

	ll, err := nt.ListTables()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ll {
		log.Println(c.Name, c.Use, c.Flags, c.Flags)
	}

	// nft add rule filter divert ip protocol tcp tproxy to :50080
	nt.AddRule(&nftables.Rule{
		Table: &nftables.Table{Name: "costin", Family: nftables.TableFamilyIPv4},
		Chain: &nftables.Chain{
			Name:     "divert",
			Type:     nftables.ChainTypeFilter,
			Hooknum:  nftables.ChainHookPrerouting,
			Priority: nftables.ChainPriorityRef(-150),
		},
		Exprs: []expr.Any{
			// payload load 4b @ network header + 12 => reg 1
			&expr.Payload{
				DestRegister: 1,
				Base:         expr.PayloadBaseNetworkHeader,
				Offset:       12,
				Len:          4,
			},
			// cmp eq reg 1 0x0245a8c0
			&expr.Cmp{
				Op:       expr.CmpOpEq,
				Register: 1,
				Data:     net.ParseIP("192.168.69.2").To4(),
			},

			//	[ payload load 1b @ network header + 9 => reg 1 ]
			&expr.Payload{DestRegister: 1, Base: expr.PayloadBaseNetworkHeader, Offset: 9, Len: 1},
			//	[ cmp eq reg 1 0x00000006 ]
			&expr.Cmp{Op: expr.CmpOpEq, Register: 1, Data: []byte{unix.IPPROTO_TCP}},
			//	[ immediate reg 1 0x0000a0c3 ]
			&expr.Immediate{Register: 1, Data: binaryutil.BigEndian.PutUint16(14001)},
			//	[ tproxy ip port reg 1 ]
			&expr.TProxy{
				Family:      byte(nftables.TableFamilyIPv4),
				TableFamily: byte(nftables.TableFamilyIPv4),
				RegPort:     1,
			},
		},
	})

	if err := nt.Flush(); err != nil {
		t.Fatal(err)
	}
}
