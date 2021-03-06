package qcclient

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/yunify/hostnic-cni/pkg/errors"
	"github.com/yunify/hostnic-cni/pkg/types"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type FakeQingCloudAPI struct {
	InstanceID string
	Nics       map[string]*types.HostNic
	seq        int

	VxNets map[string]*types.VxNet
	VPC    *types.VPC

	Tags             map[string]*types.Tag
	AfterCreatingNIC func(*types.HostNic) error
}

func NewFakeQingCloudAPI(instanceID string, vpc *types.VPC) *FakeQingCloudAPI {
	return &FakeQingCloudAPI{
		InstanceID: instanceID,
		Nics:       make(map[string]*types.HostNic),
		VxNets:     make(map[string]*types.VxNet),
		VPC:        vpc,
	}
}

func generateMAC() string {
	buf := make([]byte, 6)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	// Set the local bit
	buf[0] |= 2
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}

func (f *FakeQingCloudAPI) CreateNicsAndAttach(vxnet types.VxNet, count int) ([]*types.HostNic, error) {
	var ip net.IP
	var err error
	var nics []*types.HostNic
	v := f.VxNets[vxnet.ID]
	n := v.Network.IP.To4()

	for i := 0; i < count; i++ {
		for {
			i := rand.Int31n(253) + 2
			dup := make(net.IP, len(n))
			copy(dup, n)
			dup[3] = byte(i)
			var notgood bool
			for _, nic := range f.Nics {
				if nic.Address == dup.String() {
					notgood = true
					break
				}
			}
			if !notgood {
				ip = dup
				break
			}
		}
		mac := generateMAC()
		nic := &types.HostNic{
			ID:           mac,
			VxNet:        v,
			Address:      ip.String(),
			HardwareAddr: mac,
			DeviceNumber: len(f.Nics),
			IsPrimary:    false,
		}
		f.Nics[mac] = nic
		err = f.AfterCreatingNIC(nic)
		nics = append(nics, nic)
	}
	if err != nil {
		return nil, err
	}
	return nics, nil
}

func (q *FakeQingCloudAPI) DeattachNic(nicIDs string) error {
	return nil
}

func (q *FakeQingCloudAPI) GetNodeVxnet(vxnetName string) (string, error) {
	return "", nil
}

func (f *FakeQingCloudAPI) DeleteNic(nicID string) error {
	delete(f.Nics, nicID)
	return nil
}

func (f *FakeQingCloudAPI) GetPrimaryNIC() (*types.HostNic, error) {
	for _, nic := range f.Nics {
		if nic.IsPrimary {
			return nic, nil
		}
	}
	return nil, nil
}

func (f *FakeQingCloudAPI) DeleteNics(nicIDs []string) error {
	for _, id := range nicIDs {
		err := f.DeleteNic(id)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *FakeQingCloudAPI) GetVxNet(vxNet string) (*types.VxNet, error) {
	return f.VxNets[vxNet], nil
}

func (f *FakeQingCloudAPI) GetVxNets(vxNets []string) ([]*types.VxNet, error) {
	result := make([]*types.VxNet, 0)
	for _, v := range vxNets {
		result = append(result, f.VxNets[v])
	}
	return result, nil
}

func (f *FakeQingCloudAPI) DeleteVxNet(v string) error {
	delete(f.VxNets, v)
	return nil
}

func (f *FakeQingCloudAPI) GetNics(nics []string) ([]*types.HostNic, error) {
	result := make([]*types.HostNic, 0)
	for _, id := range nics {
		result = append(result, f.Nics[id])
	}
	return result, nil
}

func (f *FakeQingCloudAPI) CreateVxNet(name string) (*types.VxNet, error) {
	ip := fmt.Sprintf("192.168.%d.0/24", rand.Int31n(255))
	_, ipnet, _ := net.ParseCIDR(ip)
	vxnet := &types.VxNet{
		ID:      fmt.Sprintf("vxnet-%s", name),
		Network: ipnet,
		Name:    name,
	}
	f.VxNets[vxnet.ID] = vxnet
	return vxnet, nil
}

func (f *FakeQingCloudAPI) GetAttachedNICs(vxnet string) ([]*types.HostNic, error) {
	result := make([]*types.HostNic, 0)
	for _, nic := range f.Nics {
		if nic.VxNet.ID == vxnet {
			result = append(result, nic)
		}
	}
	return result, nil
}

func (f *FakeQingCloudAPI) GetVPC(string) (*types.VPC, error) {
	f.VPC.VxNets, _ = f.GetVPCVxNets(f.VPC.ID)
	return f.VPC, nil
}

func (f *FakeQingCloudAPI) GetNodeVPC() (*types.VPC, error) {
	return f.GetVPC(f.InstanceID)
}

func (f *FakeQingCloudAPI) GetVPCVxNets(routeid string) ([]*types.VxNet, error) {
	result := make([]*types.VxNet, 0)
	for _, v := range f.VxNets {
		if v.RouterID == routeid {
			result = append(result, v)
		}
	}
	return result, nil
}

func (f *FakeQingCloudAPI) JoinVPC(network, vxnetID, vpcID string) error {
	_, ipnet, _ := net.ParseCIDR(network)
	f.VxNets[vxnetID].Network = ipnet
	f.VxNets[vxnetID].RouterID = vpcID
	return nil
}
func (f *FakeQingCloudAPI) LeaveVPCAndDelete(vxnetID, vpcID string) error {
	f.VxNets[vxnetID].RouterID = ""
	delete(f.VxNets, vxnetID)
	return nil
}

func (f *FakeQingCloudAPI) GetInstanceID() string {
	return f.InstanceID
}

func (f *FakeQingCloudAPI) GetTagByLabel(label string) (*types.Tag, error) {
	for _, v := range f.Tags {
		if v.Label == label {
			return v, nil
		}
	}
	return nil, errors.NewResourceNotFoundError(types.ResourceTypeTag, label)
}

func (f *FakeQingCloudAPI) TagResources(tagid string, resourceType types.ResourceType, ids ...string) error {
	if tag, ok := f.Tags[tagid]; ok {
		for _, id := range ids {
			tag.TaggedResources = append(tag.TaggedResources, &types.TaggedResource{
				ResourceID:   id,
				ResourceType: resourceType,
			})
		}
	}
	return errors.NewResourceNotFoundError(types.ResourceTypeTag, tagid)
}

func (f *FakeQingCloudAPI) GetVxNetByName(name string) (*types.VxNet, error) {
	for _, vxnet := range f.VxNets {
		if vxnet.Name == name {
			return vxnet, nil
		}
	}
	return nil, errors.NewResourceNotFoundError(types.ResourceTypeVxnet, name)
}

func (f *FakeQingCloudAPI) CreateTag(label, color string) (string, error) {
	f.Tags[label] = &types.Tag{
		Label:           label,
		ID:              label,
		TaggedResources: []*types.TaggedResource{},
	}
	return label, nil
}

func (f *FakeQingCloudAPI) GetTagByID(id string) (*types.Tag, error) {
	if tag, ok := f.Tags[id]; ok {
		return tag, nil
	}
	return nil, errors.NewResourceNotFoundError(types.ResourceTypeTag, id)
}
