// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file provides a low-level interface to a few V8 runtime objects.
// We will use this low-level interface when serializing closures to walk the scope
// chain and find the value of free variables captured by closures, as well as getting
// source-level debug information so that we can present high-quality error messages.
//
// As a side-effect of importing this file, we must enable the --allow-natives-syntax V8
// flag. This is because we are using V8 intrinsics in order to implement this module.

import * as inspector from "inspector";
import { Runtime, Session } from "inspector";
const session = new Session();
session.connect();

function randInt() {
    return Math.floor(Math.random() * Number.MAX_SAFE_INTEGER);
}

function getRemoteObjectAsync(func: Function): Promise<inspector.Runtime.RemoteObject> {
    const randName = "func" + randInt();

    (<any>global)[randName] = func;

    return new Promise((resolve, reject) => {
        session.post("Runtime.evaluate", { expression: randName }, (e, r) => {
            delete (<any>global)[randName];

            if (e) {
                reject(e);
                return;
            }

            resolve(r.result);
        });
    });
}

interface InternalProperties {
    functionLocation?: Runtime.RemoteObject;
    scopes?: Runtime.RemoteObject;
}

function getInternalPropertiesAsync(obj: Runtime.RemoteObject): Promise<InternalProperties> {
    return new Promise((resolve, reject) => {
        session.post(
            "Runtime.getProperties",
            { objectId: obj.objectId },
            (e, r: Runtime.GetPropertiesReturnType) => {
                const result: InternalProperties = { };

                if (r) {
                    result.functionLocation = getProperty(r, "[[FunctionLocation]]");
                    result.scopes = getProperty(r, "[[Scopes]]");
                }

                resolve(result);
            });
    });

    function getProperty(r: Runtime.GetPropertiesReturnType, prop: string): Runtime.RemoteObject | undefined {
        const internalProperties = r.internalProperties;
        if (internalProperties) {
            const propDesc = internalProperties.find(p => p.name === prop);
            if (propDesc) {
                return propDesc.value;
            }
        }

        return undefined;
    }
}

function getPropertiesAsync(obj: Runtime.RemoteObject): Promise<Runtime.PropertyDescriptor[]> {
    return new Promise((resolve, reject) => {
        session.post(
            "Runtime.getProperties",
            { objectId: obj.objectId },
            (e, r: Runtime.GetPropertiesReturnType) => {
                if (e) {
                    reject(e);
                }

                resolve(r.result);
            });
    });
}

export interface FunctionInfo {
    file: string;
    line: number;
    column: number;
}

export async function getFunctionInfoAsync(func: Function): Promise<FunctionInfo> {
    const remoteObject = await getRemoteObjectAsync(func);
    const properties = await getInternalPropertiesAsync(remoteObject);

    if (!properties || !properties.functionLocation || !properties.functionLocation.value) {
        return { file: "", line: 0, column: 0 };
    }

    const location: inspector.Debugger.Location = properties.functionLocation.value;
    return { file: location.scriptId, line: location.lineNumber, column: location.columnNumber! };
}

export async function lookupCapturedVariableValueAsync(
        func: Function, freeVariable: string, throwOnFailure: boolean): any {

    const remoteObject = await getRemoteObjectAsync(func);
    const properties = await getInternalPropertiesAsync(remoteObject);

    if (properties && properties.scopes) {
        const remoteScopes = properties.scopes;
        const scopesProperties = await getPropertiesAsync(remoteScopes);

        for (const scopeProps of scopesProperties) {
            const remoteScope = scopeProps.value!;
            const scopeVariables = await getPropertiesAsync(remoteScope);

            for (const scopeVar of scopeVariables) {
                if (scopeVar.name === freeVariable) {
                    return 
                }
            }
        }
    }

    // The implementation of this function is now very straightforward since the intrinsics do all of the
    // difficult work.
    const count = getFunctionScopeCount(func);
    for (let i = 0; i < count; i++) {
        const scope = getScopeForFunction(func, i);
        if (freeVariable in scope.scopeObject) {
            return scope.scopeObject[freeVariable];
        }
    }

    if (throwOnFailure) {
        throw new Error("Unexpected missing variable in closure environment: " + freeVariable);
    }

    return undefined;
}
