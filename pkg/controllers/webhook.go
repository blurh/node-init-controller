package controllers

import (
    "encoding/json"
    "log"
    "net/http"

    admissionv1 "k8s.io/api/admission/v1"
    corev1 "k8s.io/api/core/v1"
    "github.com/blurh/node-init-controller/pkg/utils"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeUnsheduleHandler struct{}

func (*NodeUnsheduleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    log.Println("access webhook req")
    admissionReview := &admissionv1.AdmissionReview{}
    err := json.NewDecoder(r.Body).Decode(admissionReview)
    if err != nil {
        http.Error(w, "could not decode request", http.StatusBadRequest)
        return
    }

    patchUnshedulable := []patchOperation{
        {
            Op:    "add",
            Path:  "/spec/unschedulable",
            Value: true,
        },
    }
    patchBytes, err := json.Marshal(&patchUnshedulable)
    if err != nil {
        log.Printf("json marsal fail: %v", err)
    }
    admissionResponse := &admissionv1.AdmissionResponse{
        UID:       admissionReview.Request.UID,
        Allowed:   true,
        Patch:     patchBytes,
        PatchType: utils.PtrTool(admissionv1.PatchTypeJSONPatch),
    }
    respAdmissionReview := &admissionv1.AdmissionReview{
        TypeMeta: metav1.TypeMeta{
            Kind:       "AdmissionReview",
            APIVersion: "admission.k8s.io/v1",
        },
        Response: admissionResponse,
    }
    err = json.NewEncoder(w).Encode(respAdmissionReview)
    if err != nil {
        log.Printf("json encode admission review fail: %v", err)
    } else {
        var node corev1.Node
        json.Unmarshal(admissionReview.Request.Object.Raw, &node)
        log.Printf("patch node: %s unschedulable success", node.Name)
    }

    return
}

type patchOperation struct {
    Op    string `json:"op"`
    Path  string `json:"path"`
    Value any    `json:"value,omitempty"`
}
