package wrapper

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	mspclient "github.com/hyperledger/fabric-sdk-go/pkg/client/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	mspapi "github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/pkg/errors"
	"github.com/securekey/fabric-examples/fabric-cli/cmd/fabric-cli/chaincode/invokeerror"
	"github.com/ypeckstadt/fabric-sdk-go-wrapper/utils"
)

const (
	IdentityTypeUser = "client"
)

// New creates a new FabricSDKWrapper service
func New() *FabricSDKWrapper {
	return &FabricSDKWrapper{}
}

// FabricSDKWrapper implementation
type FabricSDKWrapper struct {
	sdk 	*fabsdk.FabricSDK
}

// InitializeByFile creates a Hyperledger Fabric SDK instance and loads the SDK config from a file path
// The SDK is initialized per organization
func (w *FabricSDKWrapper) InitializeByFile(configFile string, orgAdmin string, orgName string) error {
	// Initialize the SDK with the configuration file
	var sdk *fabsdk.FabricSDK
	var err error

	sdk, err = fabsdk.New(config.FromFile(configFile))

	if err != nil {
		return errors.WithMessage(err, "failed to create SDK")
	}
	w.sdk = sdk

	// TODO: code currently disabled as channel and chaincode creation does not work
	//// The resource management client is responsible for managing channels (create/update channel)
	//resourceManagerClientContext := sdk.Context(fabsdk.WithUser(orgAdmin), fabsdk.WithOrg(orgName))
	//if err != nil {
	//	return errors.WithMessage(err, "failed to load Admin identity")
	//}
	//resMgmtClient, err := resmgmt.New(resourceManagerClientContext)
	//if err != nil {
	//	return errors.WithMessage(err, "failed to create channel management client from Admin identity")
	//}
	//w.admin = resMgmtClient
	//fmt.Println("Ressource management client created")

	return nil
}

// Close closes the SDK
func (w *FabricSDKWrapper) Close() {
	w.sdk.Close()
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

// Invoke executes a Hyperledger Fabric transaction
func (w *FabricSDKWrapper) Invoke(
	channelID string,
	userName string,
	orgName string,
	chaincodeID string,
	ccFunctionName string,
	args []string,
	targetEndpoints ...string,
	) (channel.Response, error) {

	// Create channel client
	channelClient, err := w.createChannelClient(channelID, userName, orgName)

	// Create invoke request
	invokeRequest := channel.Request{
		ChaincodeID: chaincodeID,
		Fcn: ccFunctionName,
		Args:  utils.AsBytes(args),
	}

	// Create a request (proposal) and send it
	response, err := channelClient.Execute(
		invokeRequest,
		channel.WithRetry(retry.DefaultChannelOpts),
		channel.WithTargetEndpoints(targetEndpoints...),
		)
	if err != nil {
		return response, invokeerror.Errorf(invokeerror.TransientError, "SendTransactionProposal return error: %v", err)
	}

	//// Wait and check transaction response - result
	//switch pb.TxValidationCode(response.TxValidationCode) {
	//	case pb.TxValidationCode_VALID:
	//		//return response.Responses[0].GetResponse().Payload, nil
	//		return response, nil
	//	case pb.TxValidationCode_DUPLICATE_TXID, pb.TxValidationCode_MVCC_READ_CONFLICT, pb.TxValidationCode_PHANTOM_READ_CONFLICT:
	//		return response, invokeerror.Wrapf(invokeerror.TransientError, errors.New("Duplicate TxID"), "invoke Error received from eventhub for TxID [%s]. Code: %s", response.TransactionID, response.TxValidationCode)
	//	default:
	//		return response, invokeerror.Wrapf(invokeerror.PersistentError, errors.New("error"), "invoke Error received from eventhub for TxID [%s]. Code: %s", response.TransactionID, response.TxValidationCode)
	//}
	return response, nil
}

// AsyncInvoke executes a Hyperledger Fabric transaction asycn
func (w *FabricSDKWrapper) AsyncInvoke(channelID string, userName string, orgName string, chaincodeID string, ccFunctionName string, args []string) (channel.Response, error) {

	// TODO implement callbackURL and remaining todos for normal invoke

	// Create channel client
	channelClient, err := w.createChannelClient(channelID, userName, orgName)

	// Create invoke request
	request := channel.Request{
		ChaincodeID: chaincodeID,
		Fcn: ccFunctionName,
		Args:  utils.AsBytes(args),
	}

	// Create a request (proposal) and send it
	response, err := channelClient.Execute(request)
	if err != nil {
		return response, invokeerror.Errorf(invokeerror.TransientError, "SendTransactionProposal return error: %v", err)
	}

	return response, nil
}

// Query executes a Hyperledger Fabric query
func (w *FabricSDKWrapper) Query(channelID string, userName string,orgName string, chaincodeID string, ccFunctionName string, args []string, targetEndpoints ...string) (channel.Response, error) {
	channelClient, err := w.createChannelClient(channelID, userName, orgName)

	if err != nil {
		return channel.Response{}, err
	}

	response, err := channelClient.Query(
		channel.Request{
			ChaincodeID: chaincodeID,
			Fcn:         ccFunctionName,
			Args:        utils.AsBytes(args),
		},
		channel.WithRetry(retry.DefaultChannelOpts),
		channel.WithTargetEndpoints(targetEndpoints...),
		)

	if err != nil {
		return response, err
	}

	return response, nil
}

// EnrollUser enrolls a new Fabric CA user
func (w *FabricSDKWrapper) EnrollUser(userName string, orgName string) error {
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

// RemoveEnrolledUser removes a Hyperledger Fabric CA user
// by default it is not possible to remove users
func (w *FabricSDKWrapper) RemoveEnrolledUser(userName string, orgName string) (*msp.IdentityResponse, error) {
	ctxProvider := w.sdk.Context()

	mspClient, err := msp.New(ctxProvider, msp.WithOrg(orgName))
	if err != nil {
		return nil, err
	}

	return mspClient.RemoveIdentity(&msp.RemoveIdentityRequest{
		ID:userName,
		Force:true,
	})
}

// ReEnrollUser re-enrolls a Hyperledger Fabric CA user
func (w *FabricSDKWrapper) ReEnrollUser(enrollmentID string, orgName string)  error {
	ctxProvider := w.sdk.Context()
	mspClient, err := msp.New(ctxProvider, msp.WithOrg(orgName))
	if err != nil {
		return err
	}

	return mspClient.Reenroll(enrollmentID)
}

// RevokeUser revokes a Hyperledger Fabric CA user
func (w *FabricSDKWrapper) RevokeUser(username string, orgName string, caName string, reason string)  (*mspclient.RevocationResponse, error) {
	ctxProvider := w.sdk.Context()

	mspClient, err := msp.New(ctxProvider, msp.WithOrg(orgName))
	if err != nil {
		return nil, err
	}

	revokeRequest := mspclient.RevocationRequest{
		Name: username,
		Reason: reason,
		CAName: caName,
	}
	return mspClient.Revoke(&revokeRequest)
}

// GetPayloadFromResponse returns the payload from the provided channel response
func (w *FabricSDKWrapper) GetPayloadFromResponse(response *channel.Response) []byte {
	return response.Responses[0].GetResponse().Payload
}

// createMspClient creates the MSP client for the identity and organization
func (w *FabricSDKWrapper) createMSPClient(orgName string) (*mspclient.Client, error) {
	mspClient, err := mspclient.New(w.sdk.Context(), mspclient.WithOrg(orgName))
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create MSP client")
	}
	return mspClient, nil
}

// createChannelClient creates a channel client
func (w *FabricSDKWrapper) createChannelClient(channelID string, userName string, orgName string) (*channel.Client, error) {
	clientChannelContext := w.sdk.ChannelContext(channelID, fabsdk.WithUser(userName), fabsdk.WithOrg(orgName))
	client, err := channel.New(clientChannelContext)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create new channel client")
	}
	return client, nil
}

// getRegistrarEnrollmentCredentials returns the credentials for the organization's registrar user
func (w *FabricSDKWrapper) getRegistrarEnrollmentCredentials(ctxProvider context.ClientProvider) (string, string) {
	ctx, err := ctxProvider()
	if err != nil {
		return "", ""
	}

	myOrg := ctx.IdentityConfig().Client().Organization

	caID, err := w.getCAForOrganization(ctx, myOrg)


	caConfig, ok := ctx.IdentityConfig().CAConfig(caID)
	if !ok {
		return "", ""
	}

	return caConfig.Registrar.EnrollID, caConfig.Registrar.EnrollSecret
}

// getCAForOrganization returns the first found CA server for the provided organization based on the SDK configuration file
func (w *FabricSDKWrapper) getCAForOrganization(context context.Client, org string) (string, error) {
	organizations := context.EndpointConfig().NetworkConfig().Organizations
	if organizations == nil || len(organizations) == 0 {
		return "", errors.New("no organizations found")
	}
	if organizations[org].CertificateAuthorities == nil || len(organizations[org].CertificateAuthorities) == 0 {
		return "", errors.New("no certificate authorities found")
	}
	ca := organizations[org].CertificateAuthorities[0]
	return ca, errors.New("no CA found for the organization")
}
