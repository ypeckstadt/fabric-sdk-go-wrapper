package wrapper

import (
	"fmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/resmgmt"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	packager "github.com/hyperledger/fabric-sdk-go/pkg/fab/ccpackager/gopackager"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/resource"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric-sdk-go/third_party/github.com/hyperledger/fabric/protos/peer"
	"github.com/pkg/errors"
	"github.com/securekey/fabric-examples/fabric-cli/cmd/fabric-cli/chaincode/invokeerror"
	"github.com/ypeckstadt/fabric-sdk-go-wrapper/utils"
	mspapi "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
)

const (
	IdentityTypeUser = "user"
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

// InitializeByFile creates a Hyperledger Fabric SDK instance and loads the SDK config from a file path
// The SDK is initialized per organization
func (w *FabricSDKWrapper) InitializeByFile(configFile string, orgAdmin string, orgName string) error {
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

// CreateChannel creates a Hyperledger Fabric channel
func (w *FabricSDKWrapper) CreateChannel(channelID string, channelConfig string, ordererID string) error {
	adminIdentity, err := w.GetSigningIdentity("org1", "Admin")
	if err != nil {
		return err
	}
	req := resmgmt.SaveChannelRequest{ChannelID: channelID, ChannelConfigPath: channelConfig, SigningIdentities: []mspapi.SigningIdentity{adminIdentity}}
	txID, err := w.admin.SaveChannel(req, resmgmt.WithOrdererEndpoint(ordererID))
	if err != nil || txID.TransactionID == "" {
		return errors.WithMessage(err, "failed to save channel")
	}
	fmt.Println("Channel created")
	return nil
}

// JoinChannel lets the peers join the channel
func (w *FabricSDKWrapper) JoinChannel(channelID string, ordererID string) error {
	// Make admin user join the previously created channel
	if err := w.admin.JoinChannel(channelID, resmgmt.WithRetry(retry.DefaultResMgmtOpts), resmgmt.WithOrdererEndpoint(ordererID)); err != nil {
		return errors.WithMessage(err, "failed to make admin join channel")
	}
	fmt.Println("Channel joined")
	return nil
}

// Close closes the SDK
func (w *FabricSDKWrapper) Close() {
	w.sdk.Close()
}

// CreateChaincodePackage creates a Hyperledger Fabric package ready for installation
func (w *FabricSDKWrapper) CreateChaincodePackage(chaincodePath string, chaincodeGoPath string) (*resource.CCPackage, error) {
	// Create the chaincode package that will be sent to the peers
	ccPkg, err := packager.NewCCPackage(chaincodePath, chaincodeGoPath)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create chaincode package")
	}
	fmt.Println("ccPkg created")
	return ccPkg, nil
}

// GetSigningIdentity returns an organization identity
func (w *FabricSDKWrapper) GetSigningIdentity(orgName string, userName string) (mspapi.SigningIdentity, error) {
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

// InstallChaincode installs chaincode on a selected peer or all of them
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

// InstantiateChaincode instantiates selected chaincode on a channel
func (w *FabricSDKWrapper) InstantiateChaincode(channelID string, chaincodeID string,chaincodeGoPath string, version string, ccPolicy *common.SignaturePolicyEnvelope) error {
	resp, err := w.admin.InstantiateCC(channelID, resmgmt.InstantiateCCRequest{Name: chaincodeID, Path: chaincodeGoPath, Version: version, Args: [][]byte{[]byte("init")}, Policy: ccPolicy})
	if err != nil || resp.TransactionID == "" {
		return errors.WithMessage(err, "failed to instantiate the chaincode")
	}
	fmt.Println("Chaincode instantiated")
	return nil
}

// Invoke executes a Hyperledger Fabric transaction
func (w *FabricSDKWrapper) Invoke(channelID string, userName string, chaincodeID string, ccFunctionName string, args []string) ([]byte, error) {

	// TODO
	// transient map data
	// listen for chaincode events
	// selection provider options, currently static
	// TODO set target of the transaction

	// Prepend chaincode function name  to argument list
	args = append([]string{ccFunctionName}, args...)

	// Add data that will be visible in the proposal, like a description of the invoke request
	transientDataMap := make(map[string][]byte)

	// Create channel client
	channelClient, err := w.createChannelClient(channelID, userName)

	// Create invoke request
	request := channel.Request{
		ChaincodeID: chaincodeID,
		Fcn: "invoke",
		Args:  utils.AsBytes(args),
		TransientMap: transientDataMap,
	}

	// Create a request (proposal) and send it
	response, err := channelClient.Execute(request)
	if err != nil {
		return nil, fmt.Errorf("failed to move funds: %v", err)
	}

	// Wait and check transaction response - result
	switch pb.TxValidationCode(response.TxValidationCode) {
		case pb.TxValidationCode_VALID:
			return response.Responses[0].GetResponse().Payload, nil
		case pb.TxValidationCode_DUPLICATE_TXID, pb.TxValidationCode_MVCC_READ_CONFLICT, pb.TxValidationCode_PHANTOM_READ_CONFLICT:
			return nil, invokeerror.Wrapf(invokeerror.TransientError, errors.New("Duplicate TxID"), "invoke Error received from eventhub for TxID [%s]. Code: %s", response.TransactionID, response.TxValidationCode)
		default:
			return nil, invokeerror.Wrapf(invokeerror.PersistentError, errors.New("error"), "invoke Error received from eventhub for TxID [%s]. Code: %s", response.TransactionID, response.TxValidationCode)
	}
	return nil, nil
}

// Query executes a Hyperledger Fabric query
func (w *FabricSDKWrapper) Query(channelID string, userName string, chaincodeID string, ccFunctionName string, args []string) ([]byte, error) {
	// Prepend chaincode function name  to argument list
	args = append([]string{ccFunctionName}, args...)

	// Create channel client
	channelClient, err := w.createChannelClient(channelID, userName)
	if (err != nil) {
		return nil, err
	}

	if response, err := channelClient.Query(
		channel.Request{
			ChaincodeID: chaincodeID,
			Fcn:         "invoke",
			Args:        utils.AsBytes(args),
		},
	); err != nil {
		return nil, err
	} else {
		return response.Payload, nil
	}
	return nil, nil
}

// EnrollUser enrolls a new Fabric CA user
func (w *FabricSDKWrapper) EnrollUser(userName string, orgName string) (error) {
	ctxProvider := w.sdk.Context()
	mspClient, err := msp.New(ctxProvider)
	if err != nil {
		return err
	}

	// check if the user is already registered or not
	_, err = mspClient.GetIdentity(userName)
	if err == nil {
		return errors.Errorf("identity %s is already registered", userName)
	}

	// we have to enroll the CA registrar first. Otherwise,
	// CA operations that require the registrar's identity
	// will be rejected by the CA.
	// first we check if this user is already registered or not
	registrarEnrollID, registrarEnrollSecret := w.getRegistrarEnrollmentCredentials(ctxProvider)
	_, err = mspClient.GetIdentity(registrarEnrollID)
	// if error it means the identity was not found and we need to register it and enroll it
	if err != nil {
		// The enrollment process generates a new private key and
		// enrollment certificate for the user. The private key
		// is stored in the SDK crypto provider's key store, while the
		// enrollment certificate is stored in the SKD's user store
		// (state store). The CAClient will lookup the
		// registrar's identity information in these stores.
		err = mspClient.Enroll(registrarEnrollID, msp.WithSecret(registrarEnrollSecret))
		if err != nil {
			return err
		}
	}

	// Register the new user
	enrollmentSecret, err := mspClient.Register(&msp.RegistrationRequest{
		Name:       userName,
		Type:       IdentityTypeUser,
		Affiliation: orgName,
	})
	if err != nil {
		return err
	}

	// Enroll the new user
	err = mspClient.Enroll(userName, msp.WithSecret(enrollmentSecret))
	if err != nil {
		return err
	}
	return nil
}

// GetEnrolledUser returns an enrolled CA user for an organization
func (w *FabricSDKWrapper) GetEnrolledUser(userName string, orgName string) (*msp.IdentityResponse, error) {
	ctxProvider := w.sdk.Context()
	// Get the Client.
	// Without WithOrg option, uses default client organization.
	mspClient, err := msp.New(ctxProvider, msp.WithOrg(orgName))
	if err != nil {
		return nil, err
	}
	return mspClient.GetIdentity(userName)
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

func (w *FabricSDKWrapper) getRegistrarEnrollmentCredentials(ctxProvider context.ClientProvider) (string, string) {
	ctx, err := ctxProvider()
	if err != nil {
		return "", ""
	}

	myOrg := ctx.IdentityConfig().Client().Organization

	caConfig, ok := ctx.IdentityConfig().CAConfig(myOrg)
	if !ok {
		return "", ""
	}

	return caConfig.Registrar.EnrollID, caConfig.Registrar.EnrollSecret
}