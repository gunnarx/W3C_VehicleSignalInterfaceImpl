/**
* (C) 2018 Volvo Cars
*
* All files and artifacts in this repository are licensed under the
* provisions of the license provided by the LICENSE file in this repository.
*
* 
* Parser utilities for a native format VSS tree.
**/

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <stdint.h>
#include <stdbool.h>
#include "vssparserutilities.h"
#include "nativeCnodeDef.h"

FILE* treeFp;
int currentDepth;
int maxTreeDepth;

int stepsInPath;
int stepOffset;

void initTreeDepth() {
    currentDepth = 0;
    maxTreeDepth = 0;
}

void updateTreeDepth(int count) {
    if (count > 0) {
        currentDepth++;
        if (currentDepth > maxTreeDepth)
            maxTreeDepth++;
    } else {
        currentDepth--;
    }
}

void printTreeDepth() {
    printf("Max depth of VSS tree = %d\n", maxTreeDepth);
}

void readCommonPart(common_node_data_t* commonData, char** name, char** descr) {
    fread(commonData, sizeof(common_node_data_t), 1, treeFp);
    *name = (char*) malloc(sizeof(char)*(commonData->nameLen+1));
    *descr = (char*) malloc(sizeof(char)*(commonData->descrLen+1));
    fread(*name, sizeof(char)*commonData->nameLen, 1, treeFp);
    (*name)[commonData->nameLen] = '\0';
printf("Name = %s\n", *name);
    fread(*descr, sizeof(char)*commonData->descrLen, 1, treeFp);
    *((*descr)+commonData->descrLen) = '\0';
printf("Description = %s\n", *descr);
printf("Children = %d\n", commonData->children);
}

void copyData(node_t* node, common_node_data_t* commonData, char* name, char* descr) {
    node->nameLen = commonData->nameLen;
    node->name = (char*) malloc(sizeof(char)*(node->nameLen+1));
    strncpy(node->name, name, commonData->nameLen);
    node->name[commonData->nameLen] = '\0';
    node->type = commonData->type;
    node->descrLen = commonData->descrLen;
    node->description = (char*) malloc(sizeof(char)*(node->descrLen+1));
    strncpy(node->description, descr, commonData->descrLen);
    node->description[commonData->descrLen] = '\0';
    node->children = commonData->children;
}

int getObjectSize(objectTypes_t objectType) {
    switch (objectType) {
        case MEDIACOLLECTION:
            return sizeof(mediaCollectionObject_t);
        case MEDIAITEM:
            return sizeof(mediaItemObject_t);
    }
    return -1;
}

void readUniqueObjectRefs(objectTypes_t objectType, void* uniqueObject) {
    switch (objectType) {
        case MEDIACOLLECTION:
        {
            mediaCollectionObject_t* mediaCollectionObject = (mediaCollectionObject_t*) uniqueObject;
            mediaCollectionObject->items = (elementRef_t*) malloc(sizeof(elementRef_t)*mediaCollectionObject->numOfItems);
            fread(mediaCollectionObject->items, sizeof(elementRef_t)*mediaCollectionObject->numOfItems, 1, treeFp);
        }
        break;
        case MEDIAITEM:
        {
//            mediaItemObject_t* mediaItemObject = (mediaItemObject_t*) uniqueObject;
//            mediaItemObject->items = (elementRef_t*) malloc(sizeof(elementRef_t)*mediaItemObject->numOfItems);
//            fread(mediaItemObject->items, sizeof(elementRef_t)*mediaItemObject->numOfItems, 1, treeFp);
        }
        break;
        default:
            printf("readUniqueObjectRefs:unknown object type = %d\n", objectType);
        break;
    }
}

/*
  Data order on file: 
  - Common part
  - Name
  - Description
  if (node_t)
    - node_t specific
	* min/max/unit/enum
  if (rbranch_node_t)
    - rbranch specific
	* childType/numOfProperties/Properties
  if (element_node_t)
    - specific according to parent child type (mapped to struct def)
*/
struct node_t* traverseAndReadNode(struct node_t* parentPtr) {
if (parentPtr != NULL)
  printf("parent node name = %s\n", parentPtr->name);
    common_node_data_t* common_data = (common_node_data_t*)malloc(sizeof(common_node_data_t));
    if (common_data == NULL) {
        printf("traverseAndReadNode: 1st malloc failed\n");
        return NULL;
    }
    updateTreeDepth(1);
    char* name;
    char* descr;
    readCommonPart(common_data, &name, &descr);
    node_t* node = NULL;
printf("Type=%d\n",common_data->type);
    switch (common_data->type) {
        case RBRANCH:
        {
            rbranch_node_t* node2 = (rbranch_node_t*) malloc(sizeof(rbranch_node_t));
            node2->parent = parentPtr;
            copyData((node_t*)node2, common_data, name, descr);
            if (common_data->children > 0)
                node2->child = (element_node_t**) malloc(sizeof(element_node_t**)*common_data->children);
            fread(&(node2->childTypeLen), sizeof(int), 1, treeFp);
            fread(&(node2->numOfProperties), sizeof(int), 1, treeFp);
            if (node2->numOfProperties > 0) {
                node2->propertyDefinition = (propertyDefinition_t*) malloc(sizeof(propertyDefinition_t)*node2->numOfProperties);
                fread(node2->propertyDefinition, sizeof(propertyDefinition_t)*node2->numOfProperties, 1, treeFp);
            }
            node = (node_t*)node2;
        }
        break;
        case ELEMENT:
        {
            element_node_t* node2 = (element_node_t*) malloc(sizeof(element_node_t));
            node2->parent = parentPtr;
            copyData((node_t*)node2, common_data, name, descr);
            objectTypes_t objectType;
            fread(&objectType, sizeof(int), 1, treeFp);
            int objectSize = getObjectSize(objectType);
            if (objectSize > 0) {
                node2->uniqueObject = (void*) malloc(objectSize);
                *((int*)node2->uniqueObject) = objectType;
                fread(node2->uniqueObject+sizeof(int), objectSize-sizeof(int), 1, treeFp);
                readUniqueObjectRefs(objectType, node2->uniqueObject);
            }
            node = (node_t*)node2;
        }
        break;
        default:
        {
            node_t* node2 = (node_t*) malloc(sizeof(node_t));
            node2->parent = parentPtr;
            copyData((node_t*)node2, common_data, name, descr);
            if (node2->children > 0)
                node2->child = (node_t**) malloc(sizeof(node_t**)*node2->children);
            fread(&(node2->datatype), sizeof(int), 1, treeFp);
            fread(&(node2->min), sizeof(int), 1, treeFp);
            fread(&(node2->max), sizeof(int), 1, treeFp);
            fread(&(node2->unitLen), sizeof(int), 1, treeFp);
            node2->unit = NULL;
            if (node2->unitLen > 0) {
                node2->unit = (char*) malloc(sizeof(char)*(node2->unitLen+1));
                fread(node2->unit, sizeof(char)*node2->unitLen, 1, treeFp);
                node2->unit[node2->unitLen] = '\0';
            }
if (node2->unitLen > 0)
    printf("Unit = %s\n", node2->unit);
            fread(&(node2->numOfEnumElements), sizeof(int), 1, treeFp);
            if (node2->numOfEnumElements > 0) {
                node2->enumeration = (enum_t*) malloc(sizeof(enum_t)*node2->numOfEnumElements);
                fread(node2->enumeration, sizeof(enum_t)*node2->numOfEnumElements, 1, treeFp);
            }
for (int i = 0 ; i < node2->numOfEnumElements ; i++)
  printf("Enum[%d]=%s\n", i, (char*)node2->enumeration[i]);
            fread(&(node2->functionLen), sizeof(int), 1, treeFp);
            node2->function = NULL;
            if (node2->functionLen > 0) {
                node2->function = (char*) malloc(sizeof(char)*(node2->functionLen+1));
                fread(node2->function, sizeof(char)*node2->functionLen, 1, treeFp);
                node2->function[node2->functionLen] = '\0';
            }
if (node2->functionLen > 0)
    printf("Function = %s\n", node2->function);
            node = (node_t*)node2;
        }
        break;
    } //switch
    free(common_data);
    free(name);
    free(descr);
printf("node->children = %d\n", node->children);
    int childNo = 0;
    while(childNo < node->children) {
        node->child[childNo++] = traverseAndReadNode(node);
    }
    updateTreeDepth(-1);
    return node;
}

long VSSReadTree(char* filePath) {
    treeFp = fopen(filePath, "r");
    if (treeFp == NULL) {
        printf("Could not open file for reading tree data\n");
        return 0;
    }
    initTreeDepth();
    intptr_t root = (intptr_t)traverseAndReadNode(NULL);
    printTreeDepth();
    fclose(treeFp);
    return (long)root;
}

char tmpNodeName[MAXNAMELEN];
char* getNodeName(int stepNo, char* path) {
    tmpNodeName[0] = '\0';
    char* ptr = strchr(path, '.');
    if (ptr != NULL) {
        if (stepNo == 0) {
            strncpy(tmpNodeName, path, (int)(ptr-path));
            tmpNodeName[(int)(ptr-path)] = '\0';
            return tmpNodeName;
        }
        char* front;
        for (int i = 0 ; i < stepNo ; i++) {
            front = ptr+1;
            ptr = strchr(ptr+1, '.');
            if (ptr == NULL) {
                if (i == stepNo-1) {
                    ptr =&path[strlen(path)];
                    break;
                } else
                    return tmpNodeName;
            }
        }
        strncpy(tmpNodeName, front, (int)(ptr-front));
        tmpNodeName[(int)(ptr-front)] = '\0';
    } else {
        if (stepNo == 0)
            strncpy(tmpNodeName, path, MAXNAMELEN);
    }
printf("getNodeName:step=%d, name=%s\n", stepNo, tmpNodeName);
    return tmpNodeName;
}

int getNumOfPathSteps(char* path) {
    int numofelements = 0;
    char* ptr= strchr(path, '.');
    if (ptr == NULL) {
        return 1;
    } else {
        numofelements++;
        while (ptr != NULL) {
            numofelements++;
            ptr= strchr(ptr+1, '.');
        }
printf("getNumOfPathSteps=%d\n", numofelements);
        return numofelements;
    }
}

void copySteps(char* newPath, char* oldPath, int stepNo) {
    char* ptr = strchr(oldPath, '.');
    for (int i = 0 ; i < stepOffset+stepNo-1 ; i++) {
        if (ptr != NULL)
            ptr = strchr(ptr+1, '.');
    }
    if (ptr != NULL) {
        strncpy(newPath, oldPath, (int)(ptr - oldPath));
        newPath[(int)(ptr - oldPath)] = '\0';
    }
}

/**
* !!! First call to stepToNextNode() must be preceeeded by a call to initStepToNextNode() !!!
**/
struct node_t* stepToNextNode(struct node_t* ptr, int stepNo, char* searchPath, int maxFound, int* foundResponses, searchData_t* searchData) {
printf("ptr->name=%s, stepNo=%d, responsePaths[%d]=%s\n",ptr->name, stepNo, *foundResponses, (char*)(&(searchData[*foundResponses]))->responsePaths);
    if (*foundResponses >= maxFound-1)
        return NULL; // response buffers are full
    char pathNodeName[MAXNAMELEN];
    strncpy(pathNodeName, getNodeName(stepNo, searchPath), MAXNAMELEN);
    if (stepNo == stepsInPath) {
        if (strcmp(pathNodeName, ptr->name) == 0 || strcmp(pathNodeName, "*") == 0) {
            // at matching node, so save ptr and return success
            (&(searchData[*foundResponses]))->foundNodeHandles = (long)ptr;
            (*foundResponses)++;
            return ptr;
        }
    }
    strncpy(pathNodeName, getNodeName(stepNo+1, searchPath), MAXNAMELEN);  // get name of next step in path
    if (strcmp(pathNodeName, "*") != 0) {  // try to match with one of the children
        for (int i = 0 ; i < ptr->children ; i++) {
printf("ptr->child[i]->name=%s\n", ptr->child[i]->name);
            if (strcmp(pathNodeName, ptr->child[i]->name) == 0) {
                if (strlen((char*)(&(searchData[*foundResponses]))->responsePaths) > 0) // always true?
                    strcat((char*)(&(searchData[*foundResponses]))->responsePaths, ".");
                strcat((char*)(&(searchData[*foundResponses]))->responsePaths, pathNodeName);
                return stepToNextNode(ptr->child[i], stepNo+1, searchPath, maxFound, foundResponses, searchData);
            }
        }
        return NULL;
    } else {  // wildcard, try to match with all children
        struct node_t* responsePtr = NULL;
        for (int i = 0 ; (i < ptr->children) && (*foundResponses < maxFound) ; i++) {
printf("Wildcard:ptr->child[%d]->name=%s\n", i, ptr->child[i]->name);
            strcat((&(searchData[*foundResponses]))->responsePaths, ".");
            strcat((&(searchData[*foundResponses]))->responsePaths, ptr->child[i]->name);
            struct node_t* ptr2 = stepToNextNode(ptr->child[i], stepNo+1, searchPath, maxFound, foundResponses, searchData);
            if (ptr2 == NULL) {
                copySteps((&(searchData[*foundResponses]))->responsePaths, (&(searchData[*foundResponses]))->responsePaths, stepNo+1);
            } else {
                if (i < ptr->children && (&(searchData[*foundResponses-1]))->foundNodeHandles != 0) {
                    copySteps((&(searchData[*foundResponses]))->responsePaths, (&(searchData[*foundResponses-1]))->responsePaths, stepNo+1);
                } else
                    copySteps((&(searchData[*foundResponses]))->responsePaths, (&(searchData[*foundResponses]))->responsePaths, stepNo+1);
                responsePtr = ptr2;
            }
        }
        return responsePtr;
    }
}

void initStepToNextNode(struct node_t* originalRoot, struct node_t* currentRoot, char* searchPath, searchData_t* searchData, int maxFound) {
    /* 
     * This is a workaround to the fact that with X multiple wildcards, 
     * there are "(X-1)*numberofrealresults" bogus results added.
     * It seems stepToNextNode returns non-NULL when it should not in those cases?
     * See NULL check in wildcard code in stepToNextNode. 
     */
    for (int i = 0 ; i < maxFound ; i++)
        (&(searchData[i]))->foundNodeHandles = 0;

    (&(searchData[0]))->responsePaths[0] = '\0';
    do {
        path_t tmp;
        int initialLen = strlen((&(searchData[0]))->responsePaths);
        strcpy(tmp, (&(searchData[0]))->responsePaths);
        strcpy((&(searchData[0]))->responsePaths, currentRoot->name);
        if (initialLen != 0)
            strcat((&(searchData[0]))->responsePaths, ".");
        strcat((&(searchData[0]))->responsePaths, tmp);
        currentRoot = currentRoot->parent;
    } while (currentRoot != NULL);

    for (int i = 1 ; i < maxFound ; i++)
        strcpy((&(searchData[i]))->responsePaths, (&(searchData[0]))->responsePaths);

    stepOffset = getNumOfPathSteps((&(searchData[0]))->responsePaths)-1;

    stepsInPath = getNumOfPathSteps(searchPath)-1;
}

typedef struct trailingWildCardQue_t {
    char path[MAXCHARSPATH];
    struct node_t* rootNode;
    int maxFoundLeft;
    struct trailingWildCardQue_t* next;
} trailingWildCardQue_t;

trailingWildCardQue_t* trailingWildCardQue = NULL;
int trailingWildCardQueLen = 0;

void addToWildCardQue(char* path, struct node_t* rootPtr, int maxFoundLeft) {
    trailingWildCardQue_t** lastInQue;

    if (trailingWildCardQueLen == 0)
        lastInQue = &trailingWildCardQue;
    else {
        trailingWildCardQue_t* tmp = trailingWildCardQue;
        for (int i = 0 ;  i < trailingWildCardQueLen-1 ; i++)
            tmp = tmp->next;
        lastInQue = &(tmp->next);
    }
    *lastInQue = (trailingWildCardQue_t*)malloc(sizeof(trailingWildCardQue_t));
    strcpy((*lastInQue)->path, path);
    (*lastInQue)->rootNode = rootPtr;
    (*lastInQue)->maxFoundLeft = maxFoundLeft;
    (*lastInQue)->next = NULL;
    trailingWildCardQueLen++;
}

void removeFromWildCardQue(char* path, struct node_t** rootPtr, int* maxFoundLeft) {
    trailingWildCardQue_t* quePtr = trailingWildCardQue->next;

    strcpy(path, trailingWildCardQue->path);
    *rootPtr = trailingWildCardQue->rootNode;
    *maxFoundLeft = trailingWildCardQue->maxFoundLeft;
    free(trailingWildCardQue);
    trailingWildCardQue = quePtr;
    trailingWildCardQueLen--;
}

/**
* Returns handle (and path) to all leaf nodes that are found under the node before the wildcard in the path.
**/
void trailingWildCardSearch(struct node_t* rootPtr, char* searchPath, int maxFound, int* foundResponses, searchData_t* searchData) {
    int matches = 0;
    searchData_t matchingData[MAXFOUNDNODES];
    char jobPath[MAXCHARSPATH];
    struct node_t* jobRoot;
    int jobMaxFound;
    int maxFoundLeft = maxFound;

    addToWildCardQue(searchPath, rootPtr, maxFoundLeft);
    while (trailingWildCardQueLen > 0 && *foundResponses < maxFound) {
        removeFromWildCardQue(jobPath, &jobRoot, &jobMaxFound);
        matches = 0;
        initStepToNextNode(rootPtr, jobRoot, jobPath, matchingData, maxFound);
        stepToNextNode(jobRoot, 0, jobPath, jobMaxFound, &matches, matchingData);
printf("After stepToNextNode(jobPath=%s, jobRoot->name=%s) in trailingWildCardSearch(): matches=%d\n", jobPath, jobRoot->name, matches);
        maxFoundLeft -= matches;
        for (int i = 0 ; i < matches ; i++) {
            if (*foundResponses == maxFound)
                break;
            if (getType((&(matchingData[i]))->foundNodeHandles) == BRANCH || getType((&(matchingData[i]))->foundNodeHandles) == RBRANCH) {
printf("Non-leaf node=%s\n", getName((&(matchingData[i]))->foundNodeHandles));
                strcpy(jobPath, getName((&(matchingData[i]))->foundNodeHandles));
                strcat(jobPath, ".*");
                addToWildCardQue(jobPath, (struct node_t*)((intptr_t)(&(matchingData[i]))->foundNodeHandles), maxFoundLeft);
            } else {
printf("Leaf node=%s, matchingPaths[%d]=%s, *foundResponses=%d\n", getName((&(matchingData[i]))->foundNodeHandles), i, (&(matchingData[i]))->responsePaths, *foundResponses);
                strncpy((&(searchData[*foundResponses]))->responsePaths, (&(matchingData[i]))->responsePaths, MAXCHARSPATH-1);
                (&(searchData[*foundResponses]))->foundNodeHandles = (&(matchingData[i]))->foundNodeHandles;
                (*foundResponses)++;
            }
        }
    }
}

int VSSSearchNodes(char* searchPath, long rootNode, int maxFound, searchData_t* searchData, bool wildcardAllDepths) {
    intptr_t ptr = (intptr_t)rootNode;
    int foundResponses = 0;

    if ((searchPath[strlen(searchPath)-1] == '*') && (wildcardAllDepths)) {
        trailingWildCardSearch((struct node_t*)ptr, searchPath, maxFound, &foundResponses, searchData);
    } else {
        initStepToNextNode((struct node_t*)ptr,(struct node_t*)ptr, searchPath, searchData, maxFound);
        stepToNextNode((struct node_t*)ptr, 0, searchPath, maxFound, &foundResponses, searchData);
    }

    return foundResponses;
}

// added for Go testing
int VSSSimpleSearch(char* searchPath, long rootNode, bool wildcardAllDepths) {
    searchData_t searchData[150];
//    path_t responsePaths[150];
//    long foundNodes[150];
    return VSSSearchNodes(searchPath, rootNode, 150, searchData, wildcardAllDepths);
//    return VSSSearchNodes(searchPath, rootNode, 150, responsePaths, foundNodes, wildcardAllDepths);
}
void writeCommonPart(struct node_t* node) {
    common_node_data_t* commonData = (common_node_data_t*)malloc(sizeof(common_node_data_t));
    commonData->nameLen = node->nameLen;
    commonData->type = node->type;
    commonData->descrLen = node->descrLen;
    commonData->children = node->children;
    fwrite(commonData, sizeof(common_node_data_t), 1, treeFp);
    free(commonData);
    fwrite(node->name, sizeof(char)*node->nameLen, 1, treeFp);
    fwrite(node->description, sizeof(char)*node->descrLen, 1, treeFp);
}

void writeUniqueObjectRefs(objectTypes_t objectType, void* uniqueObject) {
    switch (objectType) {
        case MEDIACOLLECTION:
        {
            mediaCollectionObject_t* mediaCollectionObject = (mediaCollectionObject_t*) uniqueObject;
            fwrite(mediaCollectionObject->items, sizeof(elementRef_t)*mediaCollectionObject->numOfItems, 1, treeFp);
        }
        break;
        case MEDIAITEM:
        {
//            mediaItemObject_t* mediaItemObject = (mediaItemObject_t*) uniqueObject;
//            fwrite(mediaItemObject->items, sizeof(elementRef_t)*mediaItemObject->numOfItems, 1, treeFp);
        }
        break;
        default:
            printf("writeUniqueObjectRefs:unknown object type = %d\n", objectType);
        break;
    }
}

void traverseAndWriteNode(struct node_t* node) {
    if (node == NULL) //not needed?
        return;
    printf("Node name = %s, type=%d\n", node->name, node->type);
    writeCommonPart(node);
    switch (node->type) {
        case RBRANCH:
        {
            rbranch_node_t* node2 = (rbranch_node_t*)node;
            fwrite(&(node2->childTypeLen), sizeof(int), 1, treeFp);
            fwrite(&(node2->numOfProperties), sizeof(int), 1, treeFp);
            if (node2->numOfProperties > 0) {
                fwrite(node2->propertyDefinition, sizeof(propertyDefinition_t)*node2->numOfProperties, 1, treeFp);
            }
        }
        break;
        case ELEMENT:
        {
            element_node_t* node2 = (element_node_t*)node;
            int objectType = ((resourceObject_t*)node2->uniqueObject)->objectType;
            int objectSize = getObjectSize(objectType);
            if (objectSize > 0) {
                fwrite(node2->uniqueObject, objectSize, 1, treeFp);
                writeUniqueObjectRefs(objectType, node2->uniqueObject);
            }
        }
        break;
        default:
        {
            fwrite(&(node->datatype), sizeof(int), 1, treeFp);
            fwrite(&(node->min), sizeof(int), 1, treeFp);
            fwrite(&(node->max), sizeof(int), 1, treeFp);
            fwrite(&(node->unitLen), sizeof(int), 1, treeFp);
            if (node->unitLen > 0)
                fwrite(node->unit, sizeof(char)*node->unitLen, 1, treeFp);
            fwrite(&(node->numOfEnumElements), sizeof(int), 1, treeFp);
            if (node->numOfEnumElements > 0) {
                fwrite(node->enumeration, sizeof(enum_t)*node->numOfEnumElements, 1, treeFp);
            }
            fwrite(&(node->functionLen), sizeof(int), 1, treeFp);
            if (node->functionLen > 0)
                fwrite(node->function, sizeof(char)*node->functionLen, 1, treeFp);
//printf("numOfEnumElements=%d, unitlen=%d, functionLen=%d\n", node->numOfEnumElements, node->unitLen, node->functionLen);
for (int i = 0 ; i < node->numOfEnumElements ; i++)
  printf("Enum[%d]=%s\n", i, (char*)node->enumeration[i]);
        }
        break;
    } //switch
    int childNo = 0;
printf("node->children = %d\n", node->children);
    while(childNo < node->children) {
         traverseAndWriteNode(node->child[childNo++]);
    }
}

void VSSWriteTree(char* filePath, int rootHandle) {
    treeFp = fopen(filePath, "w");
    if (treeFp == NULL) {
        printf("Could not open file for writing tree data\n");
        return;
    }
    traverseAndWriteNode((struct node_t*)((intptr_t)rootHandle));
    fclose(treeFp);
}


// the intptr_t castings below are needed to avoid compiler warnings

long getParent(long nodeHandle) {
    return (long)((intptr_t)((node_t*)((intptr_t)nodeHandle))->parent);
}

int getNumOfChildren(long nodeHandle) {
    return (int)((intptr_t)((node_t*)((intptr_t)nodeHandle))->children);
}

long getChild(long nodeHandle, int childNo) {
    if (getNumOfChildren(nodeHandle) > childNo)
        return (long)((intptr_t)((node_t*)((intptr_t)nodeHandle))->child[childNo]);
    return 0;
}

nodeTypes_t getType(long nodeHandle) {
    return ((node_t*)((intptr_t)nodeHandle))->type;
}

nodeTypes_t getDatatype(long nodeHandle) {
    nodeTypes_t type = getType(nodeHandle);
    if (type != BRANCH && type != RBRANCH && type != ELEMENT)
        return ((node_t*)((intptr_t)nodeHandle))->datatype;
    return -1;
}

char* getName(long nodeHandle) {
    return ((node_t*)((intptr_t)nodeHandle))->name;
}

char* getDescr(long nodeHandle) {
    return ((node_t*)((intptr_t)nodeHandle))->description;
}

int getNumOfEnumElements(long nodeHandle) {
    nodeTypes_t type = getType(nodeHandle);
    if (type != BRANCH && type != RBRANCH && type != ELEMENT)
        return ((node_t*)((intptr_t)nodeHandle))->numOfEnumElements;
    return 0;
}

char* getEnumElement(long nodeHandle, int index) {
    return ((node_t*)((intptr_t)nodeHandle))->enumeration[index];
}

char* getUnit(long nodeHandle) {
    nodeTypes_t type = getType(nodeHandle);
    if (type != BRANCH && type != RBRANCH && type != ELEMENT)
        return ((node_t*)((intptr_t)nodeHandle))->unit;
    return NULL;
}

char* getFunction(long nodeHandle) {
    nodeTypes_t type = getType(nodeHandle);
    if (type != BRANCH && type != RBRANCH && type != ELEMENT)
        return ((node_t*)((intptr_t)nodeHandle))->function;
    return NULL;
}

long getResource(long nodeHandle) {
    if (getType(nodeHandle) == ELEMENT)
        return (long)((intptr_t)((element_node_t*)((intptr_t)nodeHandle))->uniqueObject);
    return -1;
}

int getObjectType(long resourceHandle) {
    return ((resourceObject_t*)((intptr_t)resourceHandle))->objectType;
}

int getMediaCollectionNumOfItems(long resourceHandle) {
    if (getObjectType(resourceHandle) == MEDIACOLLECTION)
        return ((mediaCollectionObject_t*)((intptr_t)resourceHandle))->numOfItems;
    return -1;
}

char* getMediaCollectionItemRef(long resourceHandle, int i) {
    if (getObjectType(resourceHandle) == MEDIACOLLECTION)
        return ((mediaCollectionObject_t*)((intptr_t)resourceHandle))->items[i];
    return NULL;
}

