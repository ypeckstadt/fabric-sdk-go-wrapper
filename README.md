# fabric-sdk-go-wrapper


Code example
``` go
wrapper := wrapper.New()
    wrapper.InitializeByFile("config.yaml", "Admin", "org1")
    wrapper.CreateChannel(channelID, channelConfig, ordererID)
    wrapper.JoinChannel(channelID,ordererID)
    pkg,_ := wrapper.CreateChaincodePackage(chaincodePath, chaincodeGoPath)
    wrapper.InstallChaincode(chaincodeID, chaincodePath, pkg)

    ccPolicy := cauthdsl.SignedByAnyMember([]string{orgFullPath})
    err := wrapper.InstantiateChaincode(channelID, chaincodeID, chaincodeGoPath,"0", ccPolicy)
    if err != nil {
        fmt.Println(err)
    }

    payload, err := wrapper.Invoke(channelID, "User1", chaincodeID, "jefke", []string{
        "hello",
        "5",
    })
    //
    fmt.Println(string(payload))
    payload, err = wrapper.Query(channelID, "User1", chaincodeID, "query", []string{"hello"})
    fmt.Println(string(payload))
    err = wrapper.EnrollUser("User1", "org1")
    fmt.Println(err)
    user, err := wrapper.GetEnrolledUser("User1", "org1")
    fmt.Println(err)
    fmt.Println(user)
    wrapper.Close()
```
