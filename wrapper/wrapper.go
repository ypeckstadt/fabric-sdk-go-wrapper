package wrapper

import (
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	"github.com/pkg/errors"
)

// New creates a new FabricSDKWrapper service
func New() *FabricSDKWrapper {
	return &FabricSDKWrapper{}
}

// FabricSDKWrapper implementation
type FabricSDKWrapper struct {
	sdk 	*fabsdk.FabricSDK
	admin	*resmgmt.Client
}

func (w *FabricSDKWrapper) Initialize(configFile string, orgAdmin string, orgName string) error {
	// Initialize the SDK with the configuration file
	sdk, err := fabsdk.New(config.FromFile(configFile))
	if err != nil {
		return errors.WithMessage(err, "failed to create SDK")
	}
	w.sdk = sdk

	// The resource management client is responsible for managing channels (create/update channel)
	resourceManagerClientContext := sdk.Context(fabsdk.WithUser(orgAdmin), fabsdk.WithOrg(orgName))
	if err != nil {
		return errors.WithMessage(err, "failed to load Admin identity")
	}
	resMgmtClient, err := resmgmt.New(resourceManagerClientContext)
	if err != nil {
		return errors.WithMessage(err, "failed to create channel management client from Admin identity")
	}
	w.admin = resMgmtClient
	fmt.Println("Ressource management client created")

	return nil
}

func (w *FabricSDKWrapper) CreateChannel(channelID string, channelConfig string, ordererID string) error {
	adminIdentity, err := w.GetSigningIdentity("org1", "Admin")
	if err != nil {
		return err
	}
	req := resmgmt.SaveChannelRequest{ChannelID: channelID, ChannelConfigPath: channelConfig, SigningIdentities: []msp.SigningIdentity{adminIdentity}}
	txID, err := w.admin.SaveChannel(req, resmgmt.WithOrdererEndpoint(ordererID))
	if err != nil || txID.TransactionID == "" {
		return errors.WithMessage(err, "failed to save channel")
	}
	fmt.Println("Channel created")
	return nil
}

func (w *FabricSDKWrapper) JoinChannel(channelID string, ordererID string) error {
	// Make admin user join the previously created channel
	if err := w.admin.JoinChannel(channelID, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint(ordererID)); err != nil {
		return errors.WithMessage(err, "failed to make admin join channel")
	}
	fmt.Println("Channel joined")
	return nil
}

func (w *FabricSDKWrapper) Close() {
	w.sdk.Close()
}

func (w *FabricSDKWrapper) CreateChaincodePackage(chaincodePath string, chaincodeGoPath string) (*resource.CCPackage, error) {
	// Create the chaincode package that will be sent to the peers
	ccPkg, err := packager.NewCCPackage(chaincodePath, chaincodeGoPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create chaincode package")
	}
	fmt.Println("ccPkg created")
	return ccPkg, nil
}

func (w *FabricSDKWrapper) GetSigningIdentity(orgName string, userName string) (msp.SigningIdentity, error) {
	mspClient, err := w.createMSPClient(orgName)
	if err != nil {
		return nil, err
	}
	identity, err := mspClient.GetSigningIdentity(userName)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get admin signing identity")
	}
	return identity, nil
}

func (w *FabricSDKWrapper) InstallChaincode(chaincodeID string, chaincodePath string, ccPkg *resource.CCPackage) error {
	// Install example cc to org peers
	installCCReq := resmgmt.InstallCCRequest{Name: chaincodeID, Path: chaincodePath, Version: "0", Package: ccPkg}
	_, err := w.admin.InstallCC(installCCReq, resmgmt.WithRetry(retry.DefaultResMgmtOpts))
	if err != nil {
		return errors.WithMessage(err, "failed to install chaincode")
	}
	fmt.Println("Chaincode installed")
	return nil
}

func (w *FabricSDKWrapper) InstantiateChaincode(channelID string, chaincodeID string,chaincodeGoPath string, version string, ccPolicy *common.SignaturePolicyEnvelope) error {
	resp, err := w.admin.InstantiateCC(channelID, resmgmt.InstantiateCCRequest{Name: chaincodeID, Path: chaincodeGoPath, Version: version, Args: [][]byte{[]byte("init")}, Policy: ccPolicy})
	if err != nil || resp.TransactionID == "" {
		return errors.WithMessage(err, "failed to instantiate the chaincode")
	}
	fmt.Println("Chaincode instantiated")
	return nil
}

func (w *FabricSDKWrapper) Invoke(channelID string, userName string, chaincodeID string, ccFunctionName string, args []string) error {

	// Prepend functionName  to argument list
	args = append([]string{ccFunctionName}, args...)

	// Prepare arguments
	//var args []string
	//args = append(args, "invoke")
	//args = append(args, "invoke")
	//args = append(args, "hello")
	//args = append(args, value)

	// Add data that will be visible in the proposal, like a description of the invoke request
	transientDataMap := make(map[string][]byte)
	transientDataMap["result"] = []byte("Transient data in hello invoke")

	//reg, notifier, err := setup.event.RegisterChaincodeEvent(setup.ChainCodeID, eventID)
	//if err != nil {
	//	return "", err
	//}
	//defer setup.event.Unregister(reg)

	// Create channel client
	channelClient, err := w.createChannelClient(channelID, userName)
	//
	// Create a request (proposal) and send it
	response, err := channelClient.Execute(channel.Request{ChaincodeID: chaincodeID, Fcn: "invoke", Args: [][]byte{[]byte(args[0]), []byte(args[1]), []byte(args[2])}, TransientMap: transientDataMap})
	if err != nil {
		//return "", fmt.Errorf("failed to move funds: %v", err)
	}

	fmt.Println(response.TransactionID)
	fmt.Println(err)
	return err
	//
	// Wait for the result of the submission
	//select {
	//case ccEvent := <-notifier:
	//	fmt.Printf("Received CC event: %v\n", ccEvent)
	//case <-time.After(time.Second * 20):
	//	return "", fmt.Errorf("did NOT receive CC event for eventId(%s)", eventID)
	//}
	//
	//return string(response.TransactionID), nil
}

func (w *FabricSDKWrapper) createMSPClient(orgName string) (*mspclient.Client, error) {
	mspClient, err := mspclient.New(w.sdk.Context(), mspclient.WithOrg(orgName))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create MSP client")
	}
	return mspClient, nil
}

func (w *FabricSDKWrapper) createChannelClient(channelID string, userName string) (*channel.Client, error) {
	clientContext := w.sdk.ChannelContext(channelID, fabsdk.WithUser(userName))
	client, err := channel.New(clientContext)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create new channel client")
	}
	fmt.Println("Channel client created")
	return client, nil
}


// ChannelClient
// Channel client is used to query and execute transactions
//clientContext := setup.sdk.ChannelContext(setup.ChannelID, fabsdk.WithUser(setup.UserName))
//setup.client, err = channel.New(clientContext)
//if err != nil {
//return errors.WithMessage(err, "failed to create new channel client")
//}
//fmt.Println("Channel client created")
// Creation of the client which will enables access to our channel events
//setup.event, err = event.New(clientContext)
//if err != nil {
//return errors.WithMessage(err, "failed to create new event client")
//}
//fmt.Println("Event client created")